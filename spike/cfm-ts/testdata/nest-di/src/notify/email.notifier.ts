import { Injectable } from '@nestjs/common';
import { Notifier } from './notifier';

@Injectable()
export class EmailNotifier implements Notifier {
  async send(message: string): Promise<void> {
    console.log('email', message);
  }
}
