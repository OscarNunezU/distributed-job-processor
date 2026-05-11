import {
  Injectable,
  OnModuleDestroy,
  OnModuleInit,
  Logger,
  ServiceUnavailableException,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as amqp from 'amqplib';
import CircuitBreaker from 'opossum';
import { Job } from '../../domain/job.entity';
import { MessageBrokerPort } from '../../domain/message-broker.port';

const QUEUE_NAME = 'jobs';
const DLX_NAME = 'jobs.dlx';
const DL_QUEUE = 'jobs.dead';

// After this many ms without a response the action is counted as a failure.
// sendToQueue is sync so this guards against a frozen channel.
const CB_TIMEOUT_MS = 3_000;
// Open the circuit when ≥50% of the last requests in the rolling window fail.
const CB_ERROR_THRESHOLD_PCT = 50;
// How long to wait in OPEN state before allowing one test request (HALF-OPEN).
const CB_RESET_TIMEOUT_MS = 10_000;
// Minimum number of requests in the window before the threshold is evaluated.
const CB_VOLUME_THRESHOLD = 5;

@Injectable()
export class RabbitMQBroker implements MessageBrokerPort, OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(RabbitMQBroker.name);
  private connection!: amqp.ChannelModel;
  private channel!: amqp.Channel;
  private breaker!: CircuitBreaker<[Job], void>;

  constructor(private readonly config: ConfigService) {}

  async onModuleInit(): Promise<void> {
    const url = this.config.getOrThrow<string>('RABBITMQ_URL');
    this.connection = await this.connectWithRetry(url);
    this.channel = await this.connection.createChannel();

    // Mirror the topology declared by the worker so both services agree
    await this.channel.assertExchange(DLX_NAME, 'fanout', { durable: true });
    await this.channel.assertQueue(DL_QUEUE, { durable: true });
    await this.channel.bindQueue(DL_QUEUE, DLX_NAME, '');
    await this.channel.assertQueue(QUEUE_NAME, {
      durable: true,
      arguments: { 'x-dead-letter-exchange': DLX_NAME },
    });

    this.breaker = new CircuitBreaker(this.sendToQueue.bind(this), {
      timeout: CB_TIMEOUT_MS,
      errorThresholdPercentage: CB_ERROR_THRESHOLD_PCT,
      resetTimeout: CB_RESET_TIMEOUT_MS,
      volumeThreshold: CB_VOLUME_THRESHOLD,
    });

    this.breaker.on('open', () =>
      this.logger.warn('Circuit breaker OPEN — RabbitMQ unreachable, requests failing fast'),
    );
    this.breaker.on('halfOpen', () =>
      this.logger.warn('Circuit breaker HALF-OPEN — probing RabbitMQ recovery'),
    );
    this.breaker.on('close', () =>
      this.logger.log('Circuit breaker CLOSED — RabbitMQ recovered'),
    );

    this.logger.log('Connected to RabbitMQ');
  }

  private async connectWithRetry(url: string, maxAttempts = 10): Promise<amqp.ChannelModel> {
    let delay = 1_000;
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
      try {
        return await amqp.connect(url);
      } catch (err) {
        if (attempt === maxAttempts) throw err;
        this.logger.warn(
          `RabbitMQ connection attempt ${attempt}/${maxAttempts} failed — retrying in ${delay}ms`,
        );
        await new Promise((resolve) => setTimeout(resolve, delay));
        delay = Math.min(delay * 2, 30_000);
      }
    }
    throw new Error('unreachable');
  }

  // The action wrapped by the circuit breaker.
  // Kept as a separate method so the breaker can bind to it cleanly.
  private async sendToQueue(job: Job): Promise<void> {
    const content = Buffer.from(JSON.stringify(job));
    const sent = this.channel.sendToQueue(QUEUE_NAME, content, {
      persistent: true,
      messageId: job.id,
      contentType: 'application/json',
    });
    if (!sent) {
      throw new Error(`Job ${job.id} — channel write buffer full`);
    }
  }

  async publish(job: Job): Promise<void> {
    try {
      await this.breaker.fire(job);
      this.logger.debug(`Published job ${job.id} (type: ${job.type})`);
    } catch (err) {
      if (err instanceof Error && CircuitBreaker.isOurError(err)) {
        throw new ServiceUnavailableException(
          'Message broker temporarily unavailable — please retry in a few seconds',
        );
      }
      throw err;
    }
  }

  async onModuleDestroy(): Promise<void> {
    this.breaker?.shutdown();
    await this.channel?.close();
    await this.connection?.close();
  }
}
