import { TypeOrmModuleOptions } from '@nestjs/typeorm';
import { ConfigService } from '@nestjs/config';
import { Job } from '../../jobs/domain/job.entity';

export const databaseConfig = (config: ConfigService): TypeOrmModuleOptions => ({
  type: 'postgres',
  url: config.getOrThrow<string>('POSTGRES_DSN'),
  entities: [Job],
  synchronize: false,
  migrations: ['dist/migrations/*.js', 'src/migrations/*.ts'],
  migrationsRun: true,
  logging: config.get('NODE_ENV') !== 'production',
});
