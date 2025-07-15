import { Component, OnInit, OnDestroy } from '@angular/core';
import { ActivatedRoute, NavigationEnd, Router } from '@angular/router';
import { ProjectService } from '../../services/project/project.service';
import { Project } from '../../models/project/project';
import { CommonModule, DatePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TaskService } from '../../services/task/task.service';
import { AuthService } from '../../services/user/auth.service';
import { Subscription } from 'rxjs';
import { HttpClientModule } from '@angular/common/http';

@Component({
  selector: 'app-project-details',
  standalone: true,
  imports: [CommonModule, FormsModule, HttpClientModule],
  providers: [DatePipe],
  templateUrl: './project-details.component.html',
  styleUrls: ['./project-details.component.css']
})
export class ProjectDetailsComponent implements OnInit, OnDestroy {
  project: Project | null = null;
  tasks: any[] = [];
  isLoading = false;
  isManager: boolean = false;
  isMember: boolean = false;
  isAuthenticated: boolean = false;
  showDeleteConfirmation: boolean = false;
  errorMessage: string = '';
  successMessage: string = '';
  private subscription: Subscription = new Subscription();

  constructor(
    private route: ActivatedRoute,
    private projectService: ProjectService,
    private datePipe: DatePipe,
    private router: Router,
    private taskService: TaskService,
    private authService: AuthService
  ) {}

  ngOnInit(): void {
    this.checkUserRole();
    this.listenToRouterEvents();

    const projectId = this.route.snapshot.paramMap.get('id');
    if (projectId) {
      this.loadProjectAndTasks(projectId);
    } else {
      this.errorMessage = 'Invalid Project ID. Redirecting to the projects list.';
      setTimeout(() => {
        this.errorMessage = '';
      }, 5000);
      this.router.navigate(['/projects-list']);
    }
  }

  checkUserRole(): void {
    const role = this.authService.getUserRole();
    this.isAuthenticated = !!role;
    this.isManager = role === 'manager';
    this.isMember = role === 'member';
  }

  listenToRouterEvents(): void {
    this.subscription.add(
      this.router.events.subscribe((event) => {
        if (event instanceof NavigationEnd) {
          this.checkUserRole();
        }
      })
    );
  }

  loadProjectAndTasks(projectId: string): void {
    this.isLoading = true;
    this.projectService.getProjectById(projectId).subscribe(
      (data) => {
        this.project = data;
        this.getTasks(projectId);
      },
      (error) => {
        console.error('Error fetching project details:', error);
        this.isLoading = false;
      }
    );
  }

  getTasks(projectId: string): void {
    this.projectService.getTasksForProject(projectId).subscribe(
      (tasks) => {
        this.tasks = tasks || [];
        this.tasks.forEach(task => {
          task.dependsOn = task.dependsOn || null;
          task.dependencies = []; 
        });
  
        this.tasks.forEach(task => {
          this.taskService.getTaskDependencies(task.id).subscribe({
            next: (deps) => {
              task.dependencies = deps;
            },
            error: (err) => {
              console.error(`Failed to fetch dependencies for task ${task.id}:`, err);
            }
          });
        });
  
        this.isLoading = false;
      },
      (error) => {
        console.error('Error fetching tasks:', error);
        this.isLoading = false;
      }
    );
  }
  

  openAddMembersToTask(taskId: string): void {
    const projectId = this.project?.id;
    if (projectId) {
      this.router.navigate([`/project/${projectId}/task/${taskId}/add-members`]);
    }
  }

  viewMembersToTask(taskId: string): void {
    const projectId = this.project?.id;
    if (projectId) {
      this.router.navigate([`/project/${projectId}/task/${taskId}/members`]);
    } else {
      console.error('Project ID is not available.');
    }
  }

  getTaskDependencyTitle(task: any): string | null {
    if (task.dependsOn) {
      const dependentTask = this.tasks.find(
        t => t.id === task.dependsOn || t.id === task.dependsOn?.toString()
      );
      return dependentTask ? dependentTask.title : 'Dependency not found';
    }
    return null;
  }

  updateTaskStatus(task: any): void {
    if (!task || !task.id || !task.status) {
      this.errorMessage = 'Cannot update task status. Task data is invalid.';
      setTimeout(() => { this.errorMessage = ''; }, 5000);
      task.status = task.previousStatus;  // ⬅ vraćanje starog statusa
      return;
    }
  
    if (task.dependsOn) {
      const dependentTask = this.tasks.find(t => t.id === task.dependsOn);
      if (dependentTask && dependentTask.status === 'Pending' && task.status !== 'Pending') {
        this.errorMessage = `Cannot change status to "${task.status}" because dependent task "${dependentTask.title}" is still Pending.`;
        task.status = task.previousStatus;  // ⬅ vraćanje starog statusa
        setTimeout(() => { this.errorMessage = ''; }, 5000);
        return;
      }
    }
  
    this.taskService.updateTaskStatus(task.id, task.status).subscribe({
      next: () => {
        this.successMessage = `Status for task "${task.title}" successfully updated to "${task.status}".`;
        task.previousStatus = task.status;
        setTimeout(() => { this.successMessage = ''; }, 5000);
        this.getTasks(this.project?.id!);
      },
      error: (err: any) => {
        console.error('Error updating task status:', err);
        this.errorMessage = `Failed to update status for task "${task.title}". Please try again later.`;
  
        task.status = task.previousStatus;  
  
        setTimeout(() => { this.errorMessage = ''; }, 5000);
      }
    });
  }
  

  setDependency(toTaskId: string, fromTaskId: string | null): void {
    console.log('Setting dependency:', { toTaskId, fromTaskId });
    if (!fromTaskId) return;
  
    if (toTaskId === fromTaskId) {
      this.errorMessage = 'A task cannot depend on itself.';
      setTimeout(() => (this.errorMessage = ''), 4000);
      return;
    }
  
    this.taskService.setTaskDependency({ fromTaskId, toTaskId }).subscribe({
      next: (res) => {
        console.log('Dependency set response:', res);
        this.successMessage = 'Task dependency created successfully.';
  
        const task = this.tasks.find(t => t.id === toTaskId);
        if (task) {
          task.dependsOn = fromTaskId;
  
          this.taskService.getTaskDependencies(toTaskId).subscribe({
            next: (deps) => {
              task.dependencies = deps;
            },
            error: (err) => {
              console.error(`Failed to fetch dependencies for task ${toTaskId} after setting dependency:`, err);
            }
          });
        }
  
        setTimeout(() => (this.successMessage = ''), 4000);
      },
      error: (err) => {
        console.error('Error setting task dependency:', err);
  
        if (err.status === 409) {
          const errorMsg = typeof err.error === 'string' ? err.error : (err.error?.error || '');
          if (errorMsg.toLowerCase().includes('cycle')) {
            this.errorMessage = 'Cannot create dependency due to cycle detection.';
          } else if (errorMsg.toLowerCase().includes('dependency already exists')) {
            this.errorMessage = 'Dependency already exists between these tasks.';
          } else {
            this.errorMessage = 'Conflict error occurred.';
          }        
        } else if (err.status === 201) {
          this.successMessage = 'Task dependency created successfully (received 201).';
          this.getTasks(this.project?.id!);
        } else {
          this.errorMessage = 'Failed to create task dependency.';
        }
  
        setTimeout(() => (this.errorMessage = ''), 4000);
  
        const task = this.tasks.find(t => t.id === toTaskId);
        if (task) task.dependsOn = null;
      }
    });
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

  confirmDelete(): void {
    this.showDeleteConfirmation = true;
  }

  cancelDelete(): void {
    this.showDeleteConfirmation = false;
  }

  onStatusChangeStart(task: any): void {
    task.previousStatus = task.status;
  }
  
  deleteProject(): void {
    if (!this.project) {
      console.error('No project to delete');
      return;
    }

    this.projectService.deleteProject(this.project.id).subscribe({
      next: () => {
        this.successMessage = 'Project deleted successfully!';
        setTimeout(() => {
          this.successMessage = '';
        }, 5000);

        this.router.navigate(['/users-projects']); 
      },
      error: (err) => {
        console.error('Failed to delete project:', err);
        this.errorMessage = 'Failed to delete project. Please try again later.';
        setTimeout(() => { this.errorMessage = ''; }, 5000);
      },
    });

    this.showDeleteConfirmation = false;
  }

  ngOnDestroy(): void {
    this.subscription.unsubscribe();
  }
}

