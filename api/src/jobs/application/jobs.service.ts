import { Inject, Injectable, NotFoundException } from '@nestjs/common';
import { v4 as uuidv4 } from 'uuid';
import { Job, JobStatus } from '../domain/job.entity';
import { JOB_REPOSITORY, JobRepositoryPort } from '../domain/job.repository.port';
import { MESSAGE_BROKER, MessageBrokerPort } from '../domain/message-broker.port';
import { CreateJobDto } from './create-job.dto';

@Injectable()
export class JobsService {
  constructor(
    @Inject(JOB_REPOSITORY) private readonly jobRepository: JobRepositoryPort,
    @Inject(MESSAGE_BROKER) private readonly messageBroker: MessageBrokerPort,
  ) {}

  async createJob(dto: CreateJobDto): Promise<Job> {
    const job = new Job();
    job.id = uuidv4();
    job.type = dto.type;
    job.payload = dto.payload as unknown as Record<string, unknown>;
    job.status = JobStatus.PENDING;
    job.attempts = 0;
    job.maxAttempts = dto.maxAttempts ?? 3;

    const saved = await this.jobRepository.save(job);
    await this.messageBroker.publish(saved);

    return saved;
  }

  async getJob(id: string): Promise<Job> {
    const job = await this.jobRepository.findById(id);
    if (!job) {
      throw new NotFoundException(`Job ${id} not found`);
    }
    return job;
  }

  async listJobs(limit = 20, offset = 0): Promise<{ data: Job[]; total: number }> {
    const [data, total] = await this.jobRepository.findAll(limit, offset);
    return { data, total };
  }
}
