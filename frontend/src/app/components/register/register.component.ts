import { Component } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { HttpClient } from '@angular/common/http';

@Component({
  selector: 'app-register',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './register.component.html',
  styleUrls: ['./register.component.css']
})
export class RegisterComponent {
  registerForm: FormGroup;

  constructor(private fb: FormBuilder, private http: HttpClient) {
    this.registerForm = this.fb.group({
      name: ['', Validators.required],
      lastName: ['', Validators.required],
      username: ['', [Validators.required, Validators.minLength(3)]],
      password: ['', [Validators.required, Validators.minLength(6)]],
      email: ['', [Validators.required, Validators.email]],
      userRole: ['', Validators.required]
    });
  }

  // Metoda za submit forme
  onSubmit() {
    if (this.registerForm.valid) {
      this.http.post('http://localhost:8080/register', this.registerForm.value).subscribe({
        next: () => {
          alert('Registration successful. Check your email for confirmation link.');
          this.registerForm.reset();
        },
        error: (error) => {
          console.error('Error during registration:', error);
          alert('Registration failed.');
        },
      });
    } else {
      alert('Please fill out the form correctly.');
    }
  }
}

