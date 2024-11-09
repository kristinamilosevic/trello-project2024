import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup } from '@angular/forms';
import { TaskService } from '../../services/task/task.service';
import { Router, ActivatedRoute } from '@angular/router';

@Component({
  selector: 'app-create-task',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './add-tasks.component.html',
  styleUrls: ['./add-tasks.component.css']
})
export class AddTasksComponent {
  showForm: boolean = true; 
  taskForm: FormGroup; 

  constructor(
    private fb: FormBuilder, 
    private taskService: TaskService, 
    private router: Router,
    private route: ActivatedRoute 
  ) {
    this.taskForm = this.fb.group({
      projectId: [''],
      title: [''],
      description: [''],
      status: ['Pending']
    });

    this.route.params.subscribe(params => {
      const projectId = params['projectId'];
      if (projectId) {
        this.taskForm.get('projectId')?.setValue(projectId);
      }
    });
  }

  toggleForm() {
    this.showForm = !this.showForm;
  }

  isValidTaskData(taskData: any): boolean {
    return (
      typeof taskData.projectId === 'string' &&
      typeof taskData.title === 'string' &&
      typeof taskData.description === 'string' &&
      typeof taskData.status === 'string'
    );
  }

  onSubmit() {
    const taskData = this.taskForm.value;
  
    if (!taskData.projectId || !taskData.title || !taskData.description) {
      alert('Please fill in all required fields before submitting.');
      return; 
    }
  
    if (this.taskForm.valid) { 
      taskData.projectId = taskData.projectId.toString();
  
      if (this.isValidTaskData(taskData)) {
        this.taskService.createTask(taskData).subscribe(
          response => {
            this.taskForm.reset(); 
            this.taskForm.get('status')?.setValue('Pending');
            alert('Task successfully created!');
            this.router.navigate([`/project/${taskData.projectId}`]);
          },
          error => {
            console.error('Error while creating task.', error);
            alert('Error while creating task.');
          }
        );
      } else {
        console.error('Task data is not in the correct format:', taskData);
      }
    } else {
      console.error('Form is not valid. Status:', this.taskForm.errors);
    }
  }
}
