import {
  Body,
  Controller,
  Get,
  HttpCode,
  HttpStatus,
  Param,
  ParseIntPipe,
  ParseUUIDPipe,
  Post,
  Query,
} from '@nestjs/common';
import { CreateJobDto } from '../../application/create-job.dto';
import { JobsService } from '../../application/jobs.service';

@Controller('jobs')
export class JobsController {
  constructor(private readonly jobsService: JobsService) {}

  @Post()
  @HttpCode(HttpStatus.CREATED)
  create(@Body() dto: CreateJobDto) {
    return this.jobsService.createJob(dto);
  }

  @Get(':id')
  findOne(@Param('id', ParseUUIDPipe) id: string) {
    return this.jobsService.getJob(id);
  }

  @Get()
  findAll(
    @Query('limit', new ParseIntPipe({ optional: true })) limit = 20,
    @Query('offset', new ParseIntPipe({ optional: true })) offset = 0,
  ) {
    return this.jobsService.listJobs(limit, offset);
  }
}
