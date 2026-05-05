import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Job } from '../../domain/job.entity';
import { JobRepositoryPort } from '../../domain/job.repository.port';

@Injectable()
export class TypeOrmJobRepository implements JobRepositoryPort {
  constructor(
    @InjectRepository(Job)
    private readonly repo: Repository<Job>,
  ) {}

  save(job: Job): Promise<Job> {
    return this.repo.save(job);
  }

  findById(id: string): Promise<Job | null> {
    return this.repo.findOneBy({ id });
  }

  findAll(limit: number, offset: number): Promise<[Job[], number]> {
    return this.repo.findAndCount({
      order: { createdAt: 'DESC' },
      take: limit,
      skip: offset,
    });
  }
}
