import { MigrationInterface, QueryRunner } from 'typeorm';

export class CreateJobsTable1714000000000 implements MigrationInterface {
  async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`
      CREATE TYPE job_status AS ENUM ('pending', 'processing', 'completed', 'failed');

      CREATE TABLE jobs (
        id           UUID PRIMARY KEY,
        type         VARCHAR(100)  NOT NULL,
        payload      JSONB         NOT NULL DEFAULT '{}',
        status       job_status    NOT NULL DEFAULT 'pending',
        attempts     INT           NOT NULL DEFAULT 0,
        max_attempts INT           NOT NULL DEFAULT 3,
        error        TEXT,
        created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
        updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
      );

      CREATE INDEX idx_jobs_status   ON jobs(status);
      CREATE INDEX idx_jobs_type     ON jobs(type);
      CREATE INDEX idx_jobs_created  ON jobs(created_at DESC);
    `);
  }

  async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`
      DROP TABLE IF EXISTS jobs;
      DROP TYPE IF EXISTS job_status;
    `);
  }
}
