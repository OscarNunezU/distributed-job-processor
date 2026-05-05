import { INestApplication, ValidationPipe } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import request from 'supertest';
import { AppModule } from '../src/app.module';

describe('Jobs (e2e)', () => {
  let app: INestApplication;

  beforeAll(async () => {
    const module = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();

    app = module.createNestApplication();
    app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));
    await app.init();
  });

  afterAll(() => app.close());

  it('POST /jobs → 201 with valid payload', () =>
    request(app.getHttpServer())
      .post('/jobs')
      .send({ type: 'email', payload: { to: 'user@example.com', subject: 'Test' }, maxAttempts: 3 })
      .expect(201)
      .expect((res) => {
        expect(res.body.id).toBeDefined();
        expect(res.body.status).toBe('pending');
        expect(res.body.type).toBe('email');
      }));

  it('POST /jobs → 400 with invalid job type', () =>
    request(app.getHttpServer())
      .post('/jobs')
      .send({ type: 'unknown-type', payload: {} })
      .expect(400));

  it('GET /jobs/:id → 404 for non-existent job', () =>
    request(app.getHttpServer())
      .get('/jobs/00000000-0000-0000-0000-000000000000')
      .expect(404));
});
