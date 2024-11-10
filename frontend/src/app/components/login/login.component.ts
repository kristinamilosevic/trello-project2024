import { Component, OnInit } from '@angular/core';
import { Router, ActivatedRoute } from '@angular/router';
import { AuthService } from '../../services/user/auth.service';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { HttpClientModule } from '@angular/common/http';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [FormsModule,CommonModule, ReactiveFormsModule, HttpClientModule],
  templateUrl: './login.component.html',
  styleUrls: ['./login.component.scss']
})
export class LoginComponent implements OnInit {
  email: string = '';
  password: string = '';
  errorMessage: string = '';

  constructor(
    private authService: AuthService,
    private router: Router,
    private route: ActivatedRoute
  ) {}

  ngOnInit(): void {
    // Proveri da li postoji token u query parametrima
    this.route.queryParams.subscribe(params => {
      const token = params['token'];
      if (token) {
        // Pozovi authService da potvrdi email sa tokenom
        this.authService.confirmEmail(token).subscribe({
          next: () => {
            console.log('Email confirmed successfully');
            // Ukloni token iz URL-a nakon što je iskorišćen
            this.router.navigate([], {
              queryParams: { token: null },
              queryParamsHandling: 'merge'
            });
          },
          error: (error) => {
            console.error('Error confirming email', error);
            this.errorMessage = 'Failed to confirm email';
          }
        });
      }
    });
  }

  onSubmit(): void {
    const credentials = { email: this.email, password: this.password };
    this.authService.login(credentials).subscribe({
      next: () => {
        this.router.navigate(['/add-projects']);
      },
      error: () => {
        this.errorMessage = 'Invalid email or password';
      }
    });
  }
}
