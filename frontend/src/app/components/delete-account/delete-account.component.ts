import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { AccountService } from '../../services/delete-account/delete-account.service';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-delete-account',
  templateUrl: './delete-account.component.html',
  styleUrls: ['./delete-account.component.css'],
  standalone: true,
  imports: [CommonModule]
})
export class DeleteAccountComponent implements OnInit {
  isLoading = false;
  errorMessage: string | null = null;
  managerId: string = '';

  constructor(
    private accountService: AccountService,
    private router: Router,
    private route: ActivatedRoute
  ) {}

  ngOnInit(): void {
    // Dohvati managerId iz URL-a
    this.managerId = this.route.snapshot.paramMap.get('managerId') || '';
  }

  deleteAccount() {
    if (!this.managerId) {
      this.errorMessage = 'Manager ID is missing.';
      return;
    }

    this.isLoading = true;
    this.errorMessage = null;

    this.accountService.deleteAccount(this.managerId).subscribe({
      next: () => {
        alert('Account deleted successfully.');
      },
      error: (err: HttpErrorResponse) => {
        this.isLoading = false;
        if (err.status === 403 || err.status === 409) {
          this.errorMessage = 'Cannot delete account with active projects.';
        } else {
          this.errorMessage = 'An error occurred while deleting the account.';
        }
      }
    });
  }
}
