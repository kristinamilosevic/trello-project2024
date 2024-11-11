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
  imports: [CommonModule],
})
export class DeleteAccountComponent implements OnInit {
  isLoading = false;
  errorMessage: string | null = null;
  successMessage: string | null = null;
  userId: string = '';
  role: string = '';

  constructor(
    private accountService: AccountService,
    private router: Router
  ) {}

  ngOnInit(): void {
    const token = localStorage.getItem('token');
    if (token) {
      const decodedToken: any = jwt_decode(token);
      this.userId = decodedToken.userId; // Preuzmi ID iz tokena
      this.role = decodedToken.role; // Preuzmi ulogu iz tokena
    } else {
      this.router.navigate(['/login']);
    }
  }

  deleteAccount() {
    if (!this.userId || !this.role) {
      this.errorMessage = 'User information is missing.';
      return;
    }

    this.isLoading = true;
    this.errorMessage = null;
    this.successMessage = null;

    this.accountService.deleteAccount(this.userId, this.role).subscribe({
      next: () => {
        this.successMessage = 'Account deleted successfully!';
        setTimeout(() => {
          this.successMessage = null;
          localStorage.removeItem('token');
          this.router.navigate(['/login']);
        }, 3000);
      },
      error: (err: HttpErrorResponse) => {
        this.isLoading = false;
        if (err.status === 403 || err.status === 409) {
          this.errorMessage = 'Cannot delete account with active projects.';
        } else {
          this.errorMessage = 'An error occurred while deleting the account.';
        }
        setTimeout(() => {
          this.errorMessage = null;
        }, 3000);
      }
    });
  }
}
function jwt_decode(token: string): any {
  throw new Error('Function not implemented.');
}

