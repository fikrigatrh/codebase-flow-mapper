import { Body, Controller, Get, Param, Post } from '@nestjs/common';
import { TxService } from './tx.service';
import { CreateTxDto, Tx } from './tx.dto';

@Controller('tx')
export class TxController {
  constructor(private readonly svc: TxService) {}

  @Post()
  create(@Body() dto: CreateTxDto): Promise<Tx> {
    return this.svc.create(dto);
  }

  @Get(':id')
  find(@Param('id') id: string): Promise<Tx | undefined> {
    return this.svc.find(id);
  }
}
