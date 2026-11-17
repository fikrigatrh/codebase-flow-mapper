import { Notifier } from './notifier';

// Decoy: implements Notifier but is not wired.
export class SmsNotifier implements Notifier {
  async send(message: string): Promise<void> {
    console.log('sms', message);
  }
}
