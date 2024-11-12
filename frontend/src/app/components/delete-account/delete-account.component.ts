import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { AccountService } from '../../services/delete-account/delete-account.service';
import { CommonModule } from '@angular/common';

@Component({
  standalone : true,
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
  role: string = '';

  constructor(
    private accountService: AccountService,
    private router: Router
  ) {}

  ngOnInit(): void {
    // Proveri da li su username i role sačuvani u localStorage
    const storedUsername = localStorage.getItem('username');
    const storedRole = localStorage.getItem('role');

    if (storedUsername && storedRole) {
      this.username = storedUsername;
      this.role = storedRole;
    } else {
      console.error("User information is missing from local storage.");
      this.router.navigate(['/add-projects']);
    }
  }

  deleteAccount(): void {
    const token = localStorage.getItem('token');
    if (!this.username || !this.role || !token) {
      this.errorMessage = 'User information is missing or not authenticated.';
      return;
    }
  
    this.accountService.deleteAccount(this.username, this.role).subscribe({
      next: () => {
        this.successMessage = 'Account deleted successfully!';
        localStorage.clear();
        this.router.navigate(['/login']);
      },
      error: (err) => {
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
