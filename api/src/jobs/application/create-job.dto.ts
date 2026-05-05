import {
  IsEmail,
  IsIn,
  IsInt,
  IsNotEmpty,
  IsOptional,
  IsString,
  Max,
  Min,
  ValidateNested,
} from 'class-validator';
import { Type, TypeHelpOptions } from 'class-transformer';

const VALID_JOB_TYPES = ['email', 'report', 'data-processing'] as const;
export type JobType = (typeof VALID_JOB_TYPES)[number];

export class EmailPayloadDto {
  @IsEmail()
  to!: string;

  @IsString()
  @IsOptional()
  subject?: string;
}

export class ReportPayloadDto {
  @IsString()
  @IsNotEmpty()
  report_type!: string;
}

export class DataProcessingPayloadDto {
  @IsString()
  @IsNotEmpty()
  source!: string;
}

function resolvePayloadType(opts?: TypeHelpOptions): new () => object {
  const map: Record<string, new () => object> = {
    email: EmailPayloadDto,
    report: ReportPayloadDto,
    'data-processing': DataProcessingPayloadDto,
  };
  return map[(opts?.object as { type?: string } | undefined)?.type ?? ''] ?? Object;
}

export class CreateJobDto {
  @IsString()
  @IsIn(VALID_JOB_TYPES)
  type!: JobType;

  @ValidateNested()
  @Type(resolvePayloadType)
  payload!: EmailPayloadDto | ReportPayloadDto | DataProcessingPayloadDto;

  @IsInt()
  @Min(1)
  @Max(5)
  maxAttempts: number = 3;
}
