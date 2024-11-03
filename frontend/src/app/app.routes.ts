import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AddTasksComponent } from './components/add-tasks/add-tasks.component';
import { RemoveMembersComponent } from './components/remove-members/remove-members.component';

export const routes: Routes = [
  { path: 'add-tasks', component: AddTasksComponent }, 
  { path: 'remove-members', component: RemoveMembersComponent },
];

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule],
})
export class AppRoutingModule {}
