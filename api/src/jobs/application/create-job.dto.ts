import {
  IsIn,
  IsInt,
  IsObject,
  IsString,
  Max,
  Min,
} from 'class-validator';

const VALID_JOB_TYPES = ['email', 'report', 'data-processing'] as const;
export type JobType = (typeof VALID_JOB_TYPES)[number];

export class CreateJobDto {
  @IsString()
  @IsIn(VALID_JOB_TYPES)
  type: JobType;

  @IsObject()
  payload: Record<string, unknown>;

  @IsInt()
  @Min(1)
  @Max(5)
  maxAttempts: number = 3;
}
