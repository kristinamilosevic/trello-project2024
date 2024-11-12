import { Component } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { HttpClient, HttpClientModule } from '@angular/common/http';
import { Router } from '@angular/router';

@Component({
  selector: 'app-register',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, HttpClientModule],
  templateUrl: './register.component.html',
  styleUrls: ['./register.component.css']
})
export class RegisterComponent {
  registerForm: FormGroup;

  constructor(private fb: FormBuilder, private http: HttpClient, private router: Router) {
    this.registerForm = this.fb.group({
      name: ['', Validators.required],
      lastName: ['', Validators.required],
      username: ['', [Validators.required, Validators.minLength(3)]],
      password: ['', [Validators.required, Validators.minLength(6)]],
      email: ['', [Validators.required, Validators.email]],
      userRole: ['', Validators.required]
    });
  }

  onSubmit() {
    if (this.registerForm.valid) {
      this.http.post('http://localhost:8001/register', this.registerForm.value).subscribe({
        next: (response) => {
          console.log('Response from server:', response);
          alert('Registration successful. Check your email for the verification code.');
          // Preusmeri korisnika na verifikacionu stranicu i pošalji username kao parametar
          this.router.navigate(['/verify'], { queryParams: { username: this.registerForm.value.username } });
          this.registerForm.reset();
        },
        error: (error) => {
          console.error('Error during registration:', error);
          if (error.status === 409) {
            // Ako je status kod 409, znači da korisničko ime već postoji
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
  
}
