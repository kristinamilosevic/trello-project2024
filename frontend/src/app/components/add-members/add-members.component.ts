import { Component, OnInit } from '@angular/core';
import { ProjectMembersService } from '../../services/project-members/project-members.service';
import { Member } from '../../models/member/member.model';
import { FormsModule } from '@angular/forms';
import { HttpClientModule } from '@angular/common/http';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-add-members',
  standalone: true,
  imports: [FormsModule, HttpClientModule, CommonModule],
  templateUrl: './add-members.component.html',
  styleUrls: ['./add-members.component.css']
})
export class AddMembersComponent implements OnInit {
  members: Member[] = [];
  projectMembers: Member[] = [];
  projectId: string = '672939543b45491848ab98b3'; // Zakucan ID projekta za testiranje
  errorMessage: string = ''; // Poruka o grešci

  constructor(private projectMembersService: ProjectMembersService) {}

  ngOnInit(): void {
    if (this.isValidObjectId(this.projectId)) {
      this.fetchProjectMembers();
    } else {
      console.error('Invalid projectId format. It should be a 24-character hex string.');
    }
  }

  isValidObjectId(id: string): boolean {
    return /^[a-f\d]{24}$/i.test(id);
  }

  fetchProjectMembers() {
    this.projectMembersService.getProjectMembers(this.projectId).subscribe(
      (projectMembers) => {
        this.projectMembers = projectMembers.map(member => ({
          ...member,
          id: (member as any)._id.toString()
        }));
        this.fetchUsers();
      },
      (error) => {
        console.error('Error fetching project members:', error);
      }
    );
  }

  fetchUsers() {
    this.projectMembersService.getAllUsers().subscribe(
      (allUsers) => {
        this.members = allUsers.map(user => {
          const userId = user.id.toString();
          const isSelected = this.projectMembers.some(projMember => projMember.id === userId);
          return { ...user, selected: isSelected };
        });
      },
      (error) => {
        console.error('Error fetching users:', error);
      }
    );
  }

  addSelectedMembers() {
    this.errorMessage = ''; // Reset error message
  
    const newMembersToAdd = this.members
      .filter(member => member.selected && !this.isMemberAlreadyAdded(member))
      .map(member => member.id);
  
    if (newMembersToAdd.length === 0) {
      // If no new members are selected, set an error message and exit function
      this.errorMessage = 'No new members selected for addition.';
      return;
    }
  
    const currentMemberCount = this.projectMembers.length;
    const maxMembersAllowed = 10; // Replace with actual maximum from backend
  
    if (currentMemberCount + newMembersToAdd.length > maxMembersAllowed) {
      this.errorMessage = 'You cannot add more members than the maximum allowed.';
      return;
    }
  
    this.projectMembersService.addMembers(this.projectId, newMembersToAdd).subscribe(
      () => {
        this.errorMessage = ''; // Clear error message on success
        alert('Members added successfully!');
        this.fetchProjectMembers();
      },
      (error) => {
        console.error('Error adding members:', error);
        if (error.status === 400) {
          this.errorMessage = 'The maximum number of members on the project has been reached!';
        } else {
          this.errorMessage = 'An error occurred while adding members.';
        }
      }
    );
  }
  

  isMemberAlreadyAdded(member: Member): boolean {
    return this.projectMembers.some(existingMember => existingMember.id === member.id);
  }
}
