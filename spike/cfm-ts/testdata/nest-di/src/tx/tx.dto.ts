export class CreateTxDto {
  amount!: number;
  currency!: string;
}

export interface Tx {
  id: string;
  amount: number;
}
