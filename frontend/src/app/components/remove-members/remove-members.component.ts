import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProjectMembersService } from '../../services/project-members/project-members.service';


@Component({
  selector: 'app-remove-members',
  standalone: true, 
  imports: [CommonModule], 
  templateUrl: './remove-members.component.html',
  styleUrls: ['./remove-members.component.css']
})
export class RemoveMembersComponent implements OnInit {
  projectId = '672939543b45491848ab98b3'; //id projekta
  members: any[] = [];

  constructor(private projectMembersService: ProjectMembersService) {}

  ngOnInit(): void {
    this.loadMembers();
  }

  
  loadMembers() {
    this.projectMembersService.getProjectMembers(this.projectId).subscribe(
      (data) => {
        this.members = data;
      },
      (error) => {
        console.error('Error fetching members:', error);
      }
    );
  }


  removeMember(memberId: string) {
    this.projectMembersService.removeMember(this.projectId, memberId).subscribe(
      () => {
        this.members = this.members.filter(member => member.id !== memberId);
        alert('Member removed successfully');
      },
      (error) => {
        console.error('Error removing member:', error);
        alert('Cannot remove member assigned to an in-progress task');
      }
    );
  }
}