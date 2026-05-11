import { INestApplication, ValidationPipe } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import request from 'supertest';
import { AppModule } from '../src/app.module';

const API_KEY = 'test-api-key';
const auth = { 'x-api-key': API_KEY };

describe('Jobs (e2e)', () => {
  let app: INestApplication;

  beforeAll(async () => {
    process.env.API_KEY = API_KEY;
    const module = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();

    app = module.createNestApplication();
    app.useGlobalPipes(
      new ValidationPipe({ whitelist: true, forbidNonWhitelisted: true, transform: true }),
    );
    await app.init();
  }, 60_000);

  afterAll(async () => {
    delete process.env.API_KEY;
    await app.close();
  });

  describe('Authentication', () => {
    it('POST /jobs → 401 without API key', () =>
      request(app.getHttpServer())
        .post('/jobs')
        .send({ type: 'email', payload: { to: 'user@example.com' } })
        .expect(401));

    it('POST /jobs → 401 with wrong API key', () =>
      request(app.getHttpServer())
        .post('/jobs')
        .set('x-api-key', 'wrong-key')
        .send({ type: 'email', payload: { to: 'user@example.com' } })
        .expect(401));
  });

  describe('POST /jobs', () => {
    it('→ 201 with email job', () =>
      request(app.getHttpServer())
        .post('/jobs')
        .set(auth)
        .send({ type: 'email', payload: { to: 'user@example.com', subject: 'Test' }, maxAttempts: 3 })
        .expect(201)
        .expect((res) => {
          expect(res.body.id).toBeDefined();
          expect(res.body.status).toBe('pending');
          expect(res.body.type).toBe('email');
        }));

    it('→ 201 with report job', () =>
      request(app.getHttpServer())
        .post('/jobs')
        .set(auth)
        .send({ type: 'report', payload: { report_type: 'monthly' } })
        .expect(201)
        .expect((res) => expect(res.body.type).toBe('report')));

    it('→ 201 with data-processing job', () =>
      request(app.getHttpServer())
        .post('/jobs')
        .set(auth)
        .send({ type: 'data-processing', payload: { source: 's3://bucket/file.csv' } })
        .expect(201)
        .expect((res) => expect(res.body.type).toBe('data-processing')));

    it('→ 400 with invalid job type', () =>
      request(app.getHttpServer())
        .post('/jobs')
        .set(auth)
        .send({ type: 'unknown-type', payload: {} })
        .expect(400));

    it('→ 400 when email payload is missing required field "to"', () =>
      request(app.getHttpServer())
        .post('/jobs')
        .set(auth)
        .send({ type: 'email', payload: { subject: 'No recipient' } })
        .expect(400));

    it('→ 400 when report payload is missing required field "report_type"', () =>
      request(app.getHttpServer())
        .post('/jobs')
        .set(auth)
        .send({ type: 'report', payload: {} })
        .expect(400));
  });

  describe('GET /jobs', () => {
    it('→ 200 with paginated result shape', () =>
      request(app.getHttpServer())
        .get('/jobs')
        .set(auth)
        .expect(200)
        .expect((res) => {
          expect(Array.isArray(res.body.data)).toBe(true);
          expect(typeof res.body.total).toBe('number');
        }));
  });

  describe('GET /jobs/:id', () => {
    it('→ 200 returns the created job', async () => {
      const { body: created } = await request(app.getHttpServer())
        .post('/jobs')
        .set(auth)
        .send({ type: 'email', payload: { to: 'fetch@example.com' } });

      return request(app.getHttpServer())
        .get(`/jobs/${created.id}`)
        .set(auth)
        .expect(200)
        .expect((res) => {
          expect(res.body.id).toBe(created.id);
          expect(res.body.type).toBe('email');
          expect(['pending', 'processing', 'completed']).toContain(res.body.status);
        });
    });

    it('→ 404 for non-existent job', () =>
      request(app.getHttpServer())
        .get('/jobs/00000000-0000-0000-0000-000000000000')
        .set(auth)
        .expect(404));
  });
});
