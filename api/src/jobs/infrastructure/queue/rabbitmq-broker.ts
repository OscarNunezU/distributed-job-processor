import { Injectable, OnModuleDestroy, OnModuleInit, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as amqp from 'amqplib';
import { Job } from '../../domain/job.entity';
import { MessageBrokerPort } from '../../domain/message-broker.port';

const QUEUE_NAME = 'jobs';
const DLX_NAME = 'jobs.dlx';
const DL_QUEUE = 'jobs.dead';

@Injectable()
export class RabbitMQBroker implements MessageBrokerPort, OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(RabbitMQBroker.name);
  private connection!: amqp.ChannelModel;
  private channel!: amqp.Channel;

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

  async publish(job: Job): Promise<void> {
    const content = Buffer.from(JSON.stringify(job));
    const sent = this.channel.sendToQueue(QUEUE_NAME, content, {
      persistent: true,
      messageId: job.id,
      contentType: 'application/json',
    });

    if (!sent) {
      throw new Error(`Failed to publish job ${job.id} — channel write buffer full`);
    }

    this.logger.debug(`Published job ${job.id} (type: ${job.type})`);
  }

  async onModuleDestroy(): Promise<void> {
    await this.channel?.close();
    await this.connection?.close();
  }
}
