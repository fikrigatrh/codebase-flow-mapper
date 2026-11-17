import { Module } from '@nestjs/common';
import { TxController } from './tx/tx.controller';
import { TxService } from './tx/tx.service';
import { TxRepo } from './tx/tx.repo';
import { PgTxRepo } from './tx/pg-tx.repo';
import { NOTIFIER } from './notify/notifier';
import { EmailNotifier } from './notify/email.notifier';

@Module({
  controllers: [TxController],
  providers: [
    TxService,
    { provide: TxRepo, useClass: PgTxRepo },
    { provide: NOTIFIER, useClass: EmailNotifier },
  ],
})
export class AppModule {}
