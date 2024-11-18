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
    this.taskService.getTasksByProject(projectId).subscribe(
      (tasks) => {
        this.tasks = tasks;
        console.log('Fetched tasks:', this.tasks);
        this.tasks.forEach(task => {
          console.log(`Task: ${task.title}, DependsOn: ${task.dependsOn}`);
        });
      },
      (error) => console.error('Error fetching tasks:', error)
    );
  }
  
  getTaskDependencyTitle(task: any): string | null {
    console.log('Checking dependency for task:', task);
    
    if (task.dependsOn) {
      const dependentTask = this.tasks.find(t => t.id === task.dependsOn || t.id === task.dependsOn?.toString());
      
      if (dependentTask) {
        console.log(`Dependent task found: ${dependentTask.title}`);
        return dependentTask.title;
      } else {
        console.warn(`Dependent task not found for ID: ${task.dependsOn}`);
      }
    }
    return null;
  }
  
  
  
  
  updateTaskStatus(task: any): void {
    if (task && task.id && task.status) {
      if (task.dependsOn) {
        const dependentTask = this.tasks.find(t => t.id === task.dependsOn);
        if (dependentTask && dependentTask.status !== 'Completed' && task.status !== 'Pending') {
          alert(`Cannot change status to "${task.status}" because dependent task "${dependentTask.title}" is not completed.`);
          return;
        }
      }
  
      console.log(`Attempting to update status for task "${task.title}" to "${task.status}"`);
  
      this.taskService.updateTaskStatus(task.id, task.status).subscribe({
        next: () => {
          console.log(`Status for task "${task.title}" successfully updated to "${task.status}"`);
          this.getTasks(this.project?.id!); 
        },
        error: (err: any) => {
          console.error('Error updating task status:', err);
          alert(`Failed to update status for task "${task.title}": ${err.error || err.message}`);
        }
      });
    }
  }
  
  
}

  
  
