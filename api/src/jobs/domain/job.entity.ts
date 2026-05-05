import {
  Column,
  CreateDateColumn,
  Entity,
  PrimaryColumn,
  UpdateDateColumn,
} from 'typeorm';

export enum JobStatus {
  PENDING = 'pending',
  PROCESSING = 'processing',
  COMPLETED = 'completed',
  FAILED = 'failed',
}

@Entity('jobs')
export class Job {
  @PrimaryColumn('uuid')
  id!: string;

  @Column()
  type!: string;

  @Column('jsonb')
  payload!: Record<string, unknown>;

  @Column({ type: 'enum', enum: JobStatus, default: JobStatus.PENDING })
  status!: JobStatus;

  @Column({ default: 0 })
  attempts!: number;

  @Column({ name: 'max_attempts', default: 3 })
  maxAttempts!: number;

  @Column({ nullable: true })
  error!: string;

  @CreateDateColumn({ name: 'created_at' })
  createdAt!: Date;

  @UpdateDateColumn({ name: 'updated_at' })
  updatedAt!: Date;
}
