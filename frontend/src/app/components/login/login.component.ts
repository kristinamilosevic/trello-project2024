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
  errorMessage: string = '';
  forgotEmail: string = '';
  resetMessage: string = '';
  showForgotPassword: boolean = false;

  constructor(private authService: AuthService, private router: Router) {}

  // Funkcija za prijavljivanje korisnika
  onSubmit(): void {
    const sanitizedCredentials = this.sanitizeInput({ username: this.username, password: this.password });

    if (!this.validateInput(sanitizedCredentials.username, sanitizedCredentials.password)) {
      this.errorMessage = 'Invalid username or password format';
      return;
    }

    this.authService.login(sanitizedCredentials).subscribe({
      next: () => {
        alert('Login successful!');
        this.router.navigate(['/login']);
      },
      error: () => {
        this.errorMessage = 'Invalid username or password';
      }
    });
  }

  // Prebacivanje između login forme i "Forgot Password" sekcije
  toggleForgotPassword(): void {
    this.showForgotPassword = !this.showForgotPassword;
  }

  // Funkcija za slanje linka za reset lozinke
  sendResetLink(): void {
    if (!this.forgotEmail) {
      this.resetMessage = 'Please enter a valid email';
      return;
    }

    this.authService.sendPasswordResetLink(this.forgotEmail).subscribe({
      next: () => {
        this.resetMessage = 'Reset link sent to your email!';
      },
      error: () => {
        this.resetMessage = 'Error sending reset link';
      }
    });
  }

  // Validacija unosa
  validateInput(username: string, password: string): boolean {
    const usernameRegex = /^[a-zA-Z0-9]+$/;
    return usernameRegex.test(username) && password.length >= 6 && password.length <= 20;
  }

  // Sanitizacija unosa za sprečavanje XSS napada
  sanitizeInput(data: any) {
    return {
      username: data.username.replace(/<[^>]*>?/gm, ''),
      password: data.password.replace(/<[^>]*>?/gm, '')
    };
  }
}
