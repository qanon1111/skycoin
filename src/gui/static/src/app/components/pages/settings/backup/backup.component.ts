import { Component, OnInit } from '@angular/core';
import { WalletService } from '../../../../services/wallet.service';
import { Wallet } from '../../../../app.datatypes';
import { MatDialog, MatDialogConfig } from '@angular/material';
import { SeedModalComponent } from './seed-modal/seed-modal.component';
import { PasswordDialogComponent } from "../../../layout/password-dialog/password-dialog.component";

@Component({
  selector: 'app-backup',
  templateUrl: './backup.component.html',
  styleUrls: ['./backup.component.css']
})
export class BackupComponent implements OnInit {

  folder: string;
  wallets: Wallet[];

  constructor(
    public walletService: WalletService,
    private dialog: MatDialog,
  ) {}

  ngOnInit() {
    this.walletService.folder().subscribe(folder => this.folder = folder);
    this.walletService.all().subscribe(wallets => {
      this.wallets = wallets;
    });
  }

  get onlyEncrypted() {
    return this.wallets.filter(wallet => wallet.encrypted);
  }

  showSeed(wallet: Wallet) {
    this.dialog.open(PasswordDialogComponent).componentInstance.passwordSubmit
      .subscribe(passwordDialog => {
        this.walletService.getWalletSeed(wallet, passwordDialog.password).subscribe(seed => {
          passwordDialog.close();
          const config = new MatDialogConfig();
          config.width = '566px';
          config.data = { seed };

          this.dialog.open(SeedModalComponent, config);
        }, () => passwordDialog.error());
      });
  }
}
