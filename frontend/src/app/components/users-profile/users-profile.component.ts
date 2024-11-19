import { Component, OnInit } from '@angular/core';
import { AuthService } from '../../services/user/auth.service';
import { CommonModule } from '@angular/common'; 
import { Router, RouterModule } from '@angular/router';

export interface User {
  id?: string;
  name: string;
  lastName: string;
  username: string;
  email: string;
  password: string;
  role: string;
  isActive: boolean;
}

@Component({
  selector: 'app-users-profile',
  standalone: true,
  imports: [CommonModule, RouterModule],  
  templateUrl: './users-profile.component.html',
  styleUrls: ['./users-profile.component.css']
})
export class UsersProfileComponent implements OnInit {
  userProfile: User | null = null;
  errorMessage: string = '';

  constructor(private authService: AuthService, private router: Router) {}

  ngOnInit(): void {
    this.fetchUserProfile();
  }

  fetchUserProfile(): void {
    this.authService.getUserProfile().subscribe({
      next: (profileData) => {
        this.userProfile = profileData;  
      },
      error: (err) => {
        this.errorMessage = err;  
      }
    });
  }

  deleteAccount() {
    this.router.navigate(['/delete-account']);
  }

  changePassword() {
    this.router.navigate(['/josNista']);
  }
}
