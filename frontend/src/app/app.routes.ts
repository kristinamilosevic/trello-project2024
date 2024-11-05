

import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AddTasksComponent } from './components/add-tasks/add-tasks.component';
import { AddProjectsComponent } from './components/add-projects/add-projects.component';
import { RemoveMembersComponent } from './components/remove-members/remove-members.component';
import { AddMembersComponent } from './components/add-members/add-members.component';
import { ProjectListComponent } from './components/project-list/project-list.component';
import { ProjectDetailsComponent } from './components/project-details/project-details.component';


export const routes: Routes = [
  { path: 'add-tasks', component: AddTasksComponent },
  { path: 'remove-members', component: RemoveMembersComponent },
  { path: 'add-projects', component: AddProjectsComponent },
  { path: 'add-members', component: AddMembersComponent },
  { path: 'projects-list', component: ProjectListComponent },
  { path: 'project/:id', component: ProjectDetailsComponent }, 
  // Postavljanje glavne rute na 'projects-list'
  { path: '', redirectTo: '/projects-list', pathMatch: 'full' }

];

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule],
})
export class AppRoutingModule {}





