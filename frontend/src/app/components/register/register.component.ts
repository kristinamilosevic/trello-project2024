import { Component } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { HttpClient, HttpClientModule } from '@angular/common/http';
import { Router } from '@angular/router';
import { RecaptchaModule } from 'ng-recaptcha';


@Component({
  selector: 'app-register',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, HttpClientModule, RecaptchaModule],
  templateUrl: './register.component.html',
  styleUrls: ['./register.component.css']
})
export class RegisterComponent {
  registerForm: FormGroup;
  captchaToken: string | null = null;
  captchaResolved: boolean = false;

  constructor(private fb: FormBuilder, private http: HttpClient, private router: Router) {
    this.registerForm = this.fb.group({
      name: ['', Validators.required],
      lastName: ['', Validators.required],
      username: ['', [Validators.required, Validators.minLength(3)]],
      password: ['', [Validators.required, Validators.minLength(8), Validators.pattern('^(?=.*[A-Z])(?=.*\\d)(?=.*[!@#$%^&*.,])[A-Za-z\\d!@#$%^&*.,]{8,}$')]],
      email: ['', [Validators.required, Validators.email]],
      role: ['', Validators.required]
    });
  }

  onCaptchaResolved(token: string | null): void {
    console.log('CAPTCHA resolved with token:', token); 
    this.captchaToken = token; // Čuvanje tokena
}

  onSubmit() {
    console.log('CAPTCHA token:', this.captchaToken); 

    if (!this.captchaToken) {
        alert('Please solve the CAPTCHA first!');
        return;
    }

    const payload = {
        user: this.registerForm.value,
        captchaToken: this.captchaToken
    };

    console.log('Payload being sent to the backend:', payload); 

    // Provera validnosti forme pre slanja
    if (this.registerForm.valid) {
      this.http.post(
          'http://localhost:8000/api/users/register',
          payload, 
          { headers: { 'Content-Type': 'application/json' } } 
      ).subscribe({
          next: (response) => {
              console.log('Response from server:', response);
              alert('Registration successful. Check your email for the verification code.');

              localStorage.removeItem('_grecaptcha');
              
              this.router.navigate(['/verify']);
              localStorage.setItem('username', this.registerForm.value.username);
              this.registerForm.reset();
          },
        error: (error) => {
          console.error('Error during registration:', error);
          
          if (error.error && error.error.message) {
            alert(error.error.message); 
          } else if (error.status === 409) {
            alert('Username already exists. Please choose a different one.');
          } else {
            alert('Registration failed. Please try again.');
          }
        },
      });
    } else {
      alert('Please fill out the form correctly.');
    }
  }
  
  openLogin() {
    this.router.navigate(['/login']);
  }
}