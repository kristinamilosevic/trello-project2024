

import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AddTasksComponent } from './components/add-tasks/add-tasks.component';
import { AddProjectsComponent } from './components/add-projects/add-projects.component';
import { RemoveMembersComponent } from './components/remove-members/remove-members.component';
import { AddMembersComponent } from './components/add-members/add-members.component';
import { TaskListComponent } from './components/task-list/task-list.component';


export const routes: Routes = [
  { path: 'add-tasks', component: AddTasksComponent }, 
  { path: 'remove-members', component: RemoveMembersComponent },
  { path: 'add-projects', component: AddProjectsComponent }, 
  { path: '', redirectTo: '/add-projects', pathMatch: 'full' }, 
  { path: 'add-members', component: AddMembersComponent },
  { path: 'task-list', component: TaskListComponent },

]

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule],
})
export class AppRoutingModule {}





