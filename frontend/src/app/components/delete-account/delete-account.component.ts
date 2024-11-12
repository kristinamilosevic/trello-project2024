import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { AccountService } from '../../services/delete-account/delete-account.service';
import { CommonModule } from '@angular/common';

@Component({
  standalone: true,
  selector: 'app-delete-account',
  templateUrl: './delete-account.component.html',
  styleUrls: ['./delete-account.component.css'],
  imports: [CommonModule]
})
export class DeleteAccountComponent implements OnInit {
  isLoading = false;
  errorMessage: string | null = null;
  successMessage: string | null = null;
  username: string = '';

  constructor(
    private accountService: AccountService,
    private router: Router
  ) {}

  ngOnInit(): void {
    // Proveri da li je `username` sačuvan u localStorage
    const storedUsername = localStorage.getItem('username');
    if (storedUsername) {
      this.username = storedUsername;
    } else {
      console.error("User information is missing from local storage.");
      this.router.navigate(['/add-projects']);
    }
  }

  deleteAccount(): void {
    this.isLoading = true;
    this.accountService.deleteAccount().subscribe({
      next: () => {
        this.successMessage = 'Account deleted successfully!';
        localStorage.clear();
        setTimeout(() => this.router.navigate(['/login']), 2000);
      },
      error: (err) => {
        this.isLoading = false;
        if (err.status === 401) {
          this.errorMessage = 'Unauthorized. Please log in again.';
          localStorage.clear();
          this.router.navigate(['/login']);
        } else {
          this.errorMessage = 'Failed to delete account';
        }
      }
    });
  }
}  
