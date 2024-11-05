import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ProjectService } from '../../services/project/project.service';
import { Router } from '@angular/router';

@Component({
  selector: 'app-add-projects',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './add-projects.component.html',
  styleUrls: ['./add-projects.component.css']
})
export class AddProjectsComponent {
  projectForm: FormGroup;

  constructor(
    private fb: FormBuilder, 
    private projectService: ProjectService,
    private router: Router // Dodavanje Router servisa
  ) {
    this.projectForm = this.fb.group({
      name: ['', Validators.required],
      expectedEndDate: ['', Validators.required],
      minMembers: ['', [Validators.required, Validators.min(1)]],
      maxMembers: ['', [Validators.required, Validators.min(1)]],
    });
  }

  onSubmit() {
    if (this.projectForm.valid) {
      const projectData = {
        ...this.projectForm.value,
        expectedEndDate: new Date(this.projectForm.value.expectedEndDate).toISOString(),
      };
  
      this.projectService.createProject(projectData).subscribe(
        response => {
          console.log('Project created:', response);
          alert('Project successfully created!');
          this.projectForm.reset();
          this.router.navigate(['/projects-list']); // Redirekcija na listu projekata
        },
        error => {
          console.error('Error creating project:', error);
          
          if (error.status === 400) {
            if (error.error.includes("Expected end date must be in the future")) {
              alert("Expected end date must be in the future.");
            } else if (error.error.includes("Invalid member constraints")) {
              alert("Minimum members must be at least 1, and maximum members cannot be less than minimum members.");
            } else if (error.error.includes("Project name is required")) {
              alert("Project name is required.");
            } else {
              alert("Invalid request. Please check your input.");
            }
          } else if (error.status === 401) {
            alert("Unauthorized: Manager ID is required.");
          } else {
            alert("Failed to create project.");
          }
        }
      );
    } else {
      alert('Please fill out all required fields.');
    }
  }
  
  onTasksClick() {
    alert('Navigating to tasks page...');
  }
}
