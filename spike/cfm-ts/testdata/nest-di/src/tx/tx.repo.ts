import { CreateTxDto, Tx } from './tx.dto';

// Abstract class as the injection token: the usual Nest way to inject "an interface".
export abstract class TxRepo {
  abstract save(dto: CreateTxDto): Promise<Tx>;
  abstract find(id: string): Promise<Tx | undefined>;
}
