import { TxRepo } from './tx.repo';
import { CreateTxDto, Tx } from './tx.dto';

// Decoy: implements TxRepo but no module provides it, so it must never be a call target.
export class MemoryTxRepo extends TxRepo {
  private rows = new Map<string, Tx>();

  async save(dto: CreateTxDto): Promise<Tx> {
    const tx = { id: String(this.rows.size + 1), amount: dto.amount };
    this.rows.set(tx.id, tx);
    return tx;
  }

  async find(id: string): Promise<Tx | undefined> {
    return this.rows.get(id);
  }
}
