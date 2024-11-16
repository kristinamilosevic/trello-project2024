import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { ProjectService } from '../../services/project/project.service';
import { Project } from '../../models/project/project';
import { CommonModule, DatePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TaskService } from '../../services/task/task.service';


@Component({
  selector: 'app-project-details',
  standalone: true,
  imports: [CommonModule, FormsModule],
  providers: [DatePipe],
  templateUrl: './project-details.component.html',
  styleUrls: ['./project-details.component.css']
})
export class ProjectDetailsComponent implements OnInit {
  project: Project | null = null;
  tasks: any[] = []; 

  constructor(
    private route: ActivatedRoute,
    private projectService: ProjectService,
    private datePipe: DatePipe,
    private router: Router,
    private taskService: TaskService
  ) {}

  ngOnInit(): void {
    // Pokušaj učitavanja projectId iz rute
    const projectId = this.route.snapshot.paramMap.get('id');
    
    if (projectId) {
      console.log('Project ID fetched:', projectId);
      this.loadProjectAndTasks(projectId);
    } else {
      console.error('Project ID is undefined');
    }
  }
  
  loadProjectAndTasks(projectId: string): void {
    this.projectService.getProjectById(projectId).subscribe(
      (data) => {
        this.project = data;
        console.log('Project details fetched:', this.project);
        this.getTasks(projectId);
      },
      (error) => {
        console.error('Error fetching project details:', error);
      }
    );
  }
  

  goBack(): void {
    window.history.back();
  }

  addTask(): void {
    if (this.project) {
      this.router.navigate(['/add-tasks', { projectId: this.project.id }]); 
    }
  }

  viewMembers(): void {
    if (this.project) {
      this.router.navigate(['/remove-members', this.project.id]); 
    }
  }

  addMember(): void {
    const projectId = this.project?.id;
    if (projectId) {
      this.router.navigate([`/project/${projectId}/add-members`]);
    }
  }
  
  getTasks(projectId: string): void {
    if (!projectId) {
      console.error('Project ID is missing in getTasks()');
      return;
    }
  
    console.log('Fetching tasks for project ID:', projectId);
  
    this.projectService.getTasksForProject(projectId).subscribe(
      (tasks) => {
        this.tasks = tasks;
        console.log('Tasks fetched:', this.tasks);
      },
      (error) => console.error('Error fetching tasks:', error)
    );
  }
  
  

  updateTaskStatus(task: any): void {
    if (task && task.id && task.status) {
      console.log(`Attempting to update status for task "${task.title}" to "${task.status}"`);
  
      this.taskService.updateTaskStatus(task.id, task.status).subscribe({
        next: () => {
          console.log(`Status for task "${task.title}" successfully updated to "${task.status}"`);
          this.getTasks(this.project?.id!); // Ponovno učitaj zadatke nakon ažuriranja
        },
        error: (err: any) => console.error('Error updating task status:', err)
      });
    }
  }
  
}

  
  
