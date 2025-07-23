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
import { GraphService } from '../../services/graph/graph.service';
import { NgxGraphModule } from '@swimlane/ngx-graph';
import { NgxChartsModule } from '@swimlane/ngx-charts';
import * as shape from 'd3-shape';




@Component({
  selector: 'app-project-details',
  standalone: true,
  imports: [CommonModule, FormsModule, HttpClientModule,NgxGraphModule, NgxChartsModule],
  providers: [DatePipe],
  templateUrl: './project-details.component.html',
  styleUrls: ['./project-details.component.css']
})
export class ProjectDetailsComponent implements OnInit, OnDestroy {
  projectId!: string;
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
  graphData: any = null;
  graphNodes: any[] = [];
  graphLinks: any[] = [];
  layout: string = 'dagre';
  curve = shape.curveLinear;
  graphVisible: boolean = false;
  showGraphButton: boolean = false;


  constructor(
    private route: ActivatedRoute,
    private projectService: ProjectService,
    private datePipe: DatePipe,
    private router: Router,
    private taskService: TaskService,
    private authService: AuthService,
    private graphService: GraphService
  ) {}

  ngOnInit(): void {
    this.checkUserRole();
    this.listenToRouterEvents();
    this.curve = shape.curveLinear;
  

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

  toggleGraphVisibility(): void {
  this.graphVisible = !this.graphVisible;
}


loadGraphData(): void {
  if (!this.projectId) return;

  this.graphService.getGraph(this.projectId).subscribe({
    next: (data) => {
      const linkedTaskIds = new Set<string>();
      const links = data.edges?.map((e: any) => {
        linkedTaskIds.add(e.from);
        linkedTaskIds.add(e.to);
        return {
          source: e.to,
          target: e.from
        };
      }) || [];

      const allNodes = this.tasks.map((task: any) => ({
        id: task.id,
        label: task.title,
        description: task.description || ''
      }));

      this.graphLinks = links;
      this.graphNodes = allNodes;

      this.showGraphButton = this.graphNodes.length > 0;
    },
    error: (err) => {
      console.error('Failed to reload graph data:', err);
      this.graphNodes = [];
      this.graphLinks = [];
      this.showGraphButton = false;
    }
  });
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
  this.projectId = projectId;

  this.projectService.getProjectById(projectId).subscribe({
    next: (data) => {
      this.project = data;

      // 1. Prvo uzmi taskove
      this.projectService.getTasksForProject(projectId).subscribe(
        (tasks) => {
          this.tasks = tasks || [];
          this.tasks.forEach(task => {
            task.dependsOn = task.dependsOn || null;
            task.dependencies = []; 
          });

          // 2. Poveži ih
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

          // 3. Učitaj graph iz workflow servisa
          this.graphService.getGraph(projectId).subscribe({
            next: (data) => {
              // Povezani čvorovi
              const links = data.edges?.map((e: any) => ({
                source: e.to,
                target: e.from
              })) || [];

              this.graphLinks = links;

              // Svi taskovi postaju čvorovi, bez obzira da li su povezani
              this.graphNodes = this.tasks.map((task: any) => ({
                id: task.id,
                label: task.title,
                description: task.description || ''
              }));

              this.showGraphButton = this.graphNodes.length > 0;
              this.isLoading = false;
            },
            error: (err) => {
              console.error('Failed to load graph data:', err);
              this.graphLinks = [];
              this.graphNodes = this.tasks.map((task: any) => ({
                id: task.id,
                label: task.title,
                description: task.description || ''
              }));
              this.showGraphButton = this.graphNodes.length > 0;
              this.isLoading = false;
            }
          });

        },
        (error) => {
          console.error('Error fetching tasks:', error);
          this.isLoading = false;
        }
      );
    },
    error: (error) => {
      console.error('Error fetching project details:', error);
      this.isLoading = false;
    }
  });
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

  loadWorkflowGraph(projectId: string): void {
  this.graphService.getGraph(projectId).subscribe({
    next: (data) => {
      this.graphData = data;
      console.log('Graph data loaded:', data);
    },
    error: (err) => {
      console.error('Failed to load graph data:', err);
    }
  });
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

