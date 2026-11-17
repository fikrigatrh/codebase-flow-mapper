import { Injectable } from '@nestjs/common';
import { TxRepo } from './tx.repo';
import { CreateTxDto, Tx } from './tx.dto';

@Injectable()
export class PgTxRepo extends TxRepo {
  async save(dto: CreateTxDto): Promise<Tx> {
    const row = this.toRow(dto);
    return { id: String(row.id), amount: row.amount };
  }

  async find(id: string): Promise<Tx | undefined> {
    return undefined;
  }

  private toRow(dto: CreateTxDto) {
    return { id: Date.now(), amount: dto.amount };
  }
}
