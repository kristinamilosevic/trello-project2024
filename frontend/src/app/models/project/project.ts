export class Project {
    constructor(
      public id: string,               
      public name: string,             
      public expectedEndDate: string,  
      public minMembers: number,       
      public maxMembers: number,       
      public managerId: string         
    ) {}
  }
  