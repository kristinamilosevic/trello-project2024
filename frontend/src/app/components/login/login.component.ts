import { Component } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { AuthService } from '../../services/user/auth.service';

@Component({
  standalone: true,
  imports: [FormsModule, CommonModule],
  selector: 'app-login',
  templateUrl: './login.component.html',
  styleUrls: ['./login.component.scss']
})
export class LoginComponent {
  username: string = '';
  password: string = '';
  forgotEmail: string = '';
  errorMessage: string = '';
  successMessage: string = '';
  resetMessage: string = '';
  showForgotPassword: boolean = false;

  constructor(private authService: AuthService, private router: Router) {}

  // Funkcija za prijavu korisnika
  onSubmit(): void {
    if (!this.username || !this.password) {
      this.errorMessage = 'Please enter both username and password';

      setTimeout(() => {
        this.errorMessage = '';
      }, 3000);
      
      return;
    }
    
  
    this.authService.login({ username: this.username, password: this.password }).subscribe({
      next: (response: any) => {
        // Sačuvaj informacije u localStorage
        localStorage.setItem('username', response.username);
        localStorage.setItem('role', response.role);
        localStorage.setItem('token', response.token);
  
        this.successMessage = 'Login successful!';
        setTimeout(() => {
          this.router.navigate(['/add-projects']);
        }, 2000); // Preusmeravanje nakon 2 sekunde
      },
      error: () => {
        this.errorMessage = 'Invalid username or password';
      }
    });
  }
  

  // Funkcija za otvaranje "Forgot Password" sekcije
  openForgotPassword(): void {
    if (!this.username) {
      this.errorMessage = 'Please enter your username';
      return;
    }
    this.errorMessage = '';
    this.showForgotPassword = true;
  }

  // Funkcija za zatvaranje "Forgot Password" sekcije
  closeForgotPassword(): void {
    this.showForgotPassword = false;
    this.forgotEmail = '';
    this.resetMessage = '';
  }

  // Funkcija za slanje linka za reset lozinke
  sendResetLink(): void {
    if (!this.forgotEmail) {
      this.resetMessage = 'Please enter a valid email';
      return;
    }

    this.authService.sendPasswordResetLink(this.username, this.forgotEmail).subscribe({
      next: () => {
        this.resetMessage = 'Reset link sent to your email!';
      },
      error: () => {
        this.resetMessage = 'Reset link sent to your email!';
      }
    });
  }
}
