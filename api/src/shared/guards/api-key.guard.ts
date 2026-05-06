import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Request } from 'express';

@Injectable()
export class ApiKeyGuard implements CanActivate {
  constructor(private readonly config: ConfigService) {}

  canActivate(context: ExecutionContext): boolean {
    const expected = this.config.get<string>('API_KEY');
    if (!expected) return true;

    const request = context.switchToHttp().getRequest<Request>();
    const provided = request.headers['x-api-key'];

    if (provided !== expected) {
      throw new UnauthorizedException('Invalid or missing API key');
    }
    return true;
  }
}
