export interface Notifier {
  send(message: string): Promise<void>;
}

export const NOTIFIER = Symbol('NOTIFIER');
