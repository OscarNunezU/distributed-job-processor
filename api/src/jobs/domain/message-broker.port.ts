import { Job } from './job.entity';

export const MESSAGE_BROKER = Symbol('MESSAGE_BROKER');

export interface MessageBrokerPort {
  publish(job: Job): Promise<void>;
}
