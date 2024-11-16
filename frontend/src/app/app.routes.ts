import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AddTasksComponent } from './components/add-tasks/add-tasks.component';
import { AddProjectsComponent } from './components/add-projects/add-projects.component';
import { RemoveMembersComponent } from './components/remove-members/remove-members.component';
import { AddMembersComponent } from './components/add-members/add-members.component';
import { ProjectListComponent } from './components/project-list/project-list.component';
import { ProjectDetailsComponent } from './components/project-details/project-details.component';
import { TaskListComponent } from './components/task-list/task-list.component';
import { RegisterComponent } from './components/register/register.component';
import { DeleteAccountComponent } from './components/delete-account/delete-account.component';
import { LoginComponent } from './components/login/login.component';
import { VerifyCodeComponent } from './components/verify-code/verify-code.component';
import { UsersProjectsComponent } from './components/users-projects/users-projects.component';
import { UsersProfileComponent } from './components/users-profile/users-profile.component';



// export const routes: Routes = [
//   { path: 'add-tasks', component: AddTasksComponent },
//   { path: 'remove-members', component: RemoveMembersComponent },
//   { path: 'add-projects', component: AddProjectsComponent }, 
//   { path: '', redirectTo: '/add-projects', pathMatch: 'full' }, 
//   { path: 'add-members', component: AddMembersComponent },
//   { path: 'add-projects', component: AddProjectsComponent },
//   { path: 'add-members', component: AddMembersComponent },
//   { path: 'projects-list', component: ProjectListComponent },
//   { path: 'project/:id', component: ProjectDetailsComponent }, 
//   { path: '', redirectTo: '/projects-list', pathMatch: 'full' },
//   { path: 'add-projects', component: AddProjectsComponent }, 
//   { path: '', redirectTo: '/add-projects', pathMatch: 'full' }, 
//   { path: 'add-members', component: AddMembersComponent },
//   { path: 'task-list', component: TaskListComponent },

// ];

export const routes: Routes = [
  { path: 'add-tasks', component: AddTasksComponent },
  { path: 'remove-members', component: RemoveMembersComponent },
  { path: 'remove-members/:id', component: RemoveMembersComponent },
  { path: 'add-projects', component: AddProjectsComponent }, 
  { path: '', redirectTo: '/add-projects', pathMatch: 'full' },
  { path: 'project/:id/add-members', component: AddMembersComponent },
  { path: 'projects-list', component: ProjectListComponent },
  { path: 'project/:id', component: ProjectDetailsComponent },
  { path: 'project/:id', component: ProjectDetailsComponent }, 
  { path: '', redirectTo: '/projects-list', pathMatch: 'full' },
  { path: 'task-list', component: TaskListComponent },
  { path: '', redirectTo: '/projects-list', pathMatch: 'full' }, 
  { path: 'register', component: RegisterComponent },
  { path: 'users-projects', component: UsersProjectsComponent },
  { path: 'delete-account', component: DeleteAccountComponent },
  { path: 'login', component: LoginComponent }, 
  { path: '', redirectTo: '/login', pathMatch: 'full' },
  { path: 'verify', component: VerifyCodeComponent },
  { path: 'magic-login', component: LoginComponent },
  { path: 'users-profile', component: UsersProfileComponent }
];

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule],
})
export class AppRoutingModule {}





