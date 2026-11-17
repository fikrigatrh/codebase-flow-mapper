import { Inject, Injectable } from '@nestjs/common';
import { TxRepo } from './tx.repo';
import { CreateTxDto, Tx } from './tx.dto';
import { NOTIFIER, Notifier } from '../notify/notifier';

@Injectable()
export class TxService {
  constructor(
    private readonly repo: TxRepo,
    @Inject(NOTIFIER) private readonly notifier: Notifier,
  ) {}

  async create(dto: CreateTxDto): Promise<Tx> {
    const tx = await this.repo.save(dto);
    if (dto.amount > 1000) {
      await this.notifier.send(`large tx ${tx.id}`);
    }
    return tx;
  }

  find(id: string): Promise<Tx | undefined> {
    return this.repo.find(id);
  }
}
