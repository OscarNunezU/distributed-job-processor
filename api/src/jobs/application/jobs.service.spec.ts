import { NotFoundException } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { JOB_REPOSITORY } from '../domain/job.repository.port';
import { MESSAGE_BROKER } from '../domain/message-broker.port';
import { Job, JobStatus } from '../domain/job.entity';
import { CreateJobDto } from './create-job.dto';
import { JobsService } from './jobs.service';

const makeJob = (overrides: Partial<Job> = {}): Job =>
  Object.assign(new Job(), {
    id: 'test-uuid',
    type: 'email',
    payload: { to: 'test@example.com' },
    status: JobStatus.PENDING,
    attempts: 0,
    maxAttempts: 3,
    createdAt: new Date(),
    updatedAt: new Date(),
    ...overrides,
  });

describe('JobsService', () => {
  let service: JobsService;
  const mockRepo = {
    save: jest.fn(),
    findById: jest.fn(),
    findAll: jest.fn(),
  };
  const mockBroker = { publish: jest.fn() };

  beforeEach(async () => {
    jest.clearAllMocks();
    const module: TestingModule = await Test.createTestingModule({
      providers: [
        JobsService,
        { provide: JOB_REPOSITORY, useValue: mockRepo },
        { provide: MESSAGE_BROKER, useValue: mockBroker },
      ],
    }).compile();

    service = module.get(JobsService);
  });

  describe('createJob', () => {
    it('saves the job and publishes to the broker', async () => {
      const dto: CreateJobDto = { type: 'email', payload: { to: 'a@b.com' }, maxAttempts: 3 };
      const saved = makeJob();
      mockRepo.save.mockResolvedValue(saved);
      mockBroker.publish.mockResolvedValue(undefined);

      const result = await service.createJob(dto);

      expect(mockRepo.save).toHaveBeenCalledTimes(1);
      expect(mockBroker.publish).toHaveBeenCalledWith(saved);
      expect(result).toBe(saved);
    });

    it('does not publish if save fails', async () => {
      mockRepo.save.mockRejectedValue(new Error('db error'));
      await expect(service.createJob({ type: 'email', payload: { to: 'fail@test.com' }, maxAttempts: 3 } as CreateJobDto)).rejects.toThrow('db error');
      expect(mockBroker.publish).not.toHaveBeenCalled();
    });
  });

  describe('getJob', () => {
    it('returns the job when found', async () => {
      const job = makeJob();
      mockRepo.findById.mockResolvedValue(job);
      await expect(service.getJob('test-uuid')).resolves.toBe(job);
    });

    it('throws NotFoundException when job does not exist', async () => {
      mockRepo.findById.mockResolvedValue(null);
      await expect(service.getJob('missing')).rejects.toThrow(NotFoundException);
    });
  });

  describe('listJobs', () => {
    it('returns paginated jobs', async () => {
      const jobs = [makeJob()];
      mockRepo.findAll.mockResolvedValue([jobs, 1]);
      const result = await service.listJobs(10, 0);
      expect(result).toEqual({ data: jobs, total: 1 });
      expect(mockRepo.findAll).toHaveBeenCalledWith(10, 0);
    });
  });
});
