import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { JobsService } from './application/jobs.service';
import { Job } from './domain/job.entity';
import { JOB_REPOSITORY } from './domain/job.repository.port';
import { MESSAGE_BROKER } from './domain/message-broker.port';
import { JobsController } from './infrastructure/controllers/jobs.controller';
import { RabbitMQBroker } from './infrastructure/queue/rabbitmq-broker';
import { TypeOrmJobRepository } from './infrastructure/repositories/typeorm-job.repository';

@Module({
  imports: [TypeOrmModule.forFeature([Job])],
  controllers: [JobsController],
  providers: [
    JobsService,
    { provide: JOB_REPOSITORY, useClass: TypeOrmJobRepository },
    { provide: MESSAGE_BROKER, useClass: RabbitMQBroker },
  ],
})
export class JobsModule {}
