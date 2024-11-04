import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AddTasksComponent } from './components/add-tasks/add-tasks.component';
import { AddProjectsComponent } from './components/add-projects/add-projects.component';
import { RemoveMembersComponent } from './components/remove-members/remove-members.component';

export const routes: Routes = [
  { path: 'add-tasks', component: AddTasksComponent }, 
  { path: 'remove-members', component: RemoveMembersComponent },
  { path: 'add-projects', component: AddProjectsComponent }, // Dodavanje nove putanje za AddProjectsComponent
  { path: '', redirectTo: '/add-projects', pathMatch: 'full' }, // Opcionalno: preusmeravanje na add-projects ili drugu komponentu

]

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule],
})
export class AppRoutingModule {}
