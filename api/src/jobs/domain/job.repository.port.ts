import { Job } from './job.entity';

export const JOB_REPOSITORY = Symbol('JOB_REPOSITORY');

export interface JobRepositoryPort {
  save(job: Job): Promise<Job>;
  findById(id: string): Promise<Job | null>;
  findAll(limit: number, offset: number): Promise<[Job[], number]>;
}
