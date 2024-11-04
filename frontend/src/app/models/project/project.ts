<<<<<<< HEAD
export interface Project {
    id: string;
    name: string;
    members: { id: string; name: string }[];
  }
=======
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
  
>>>>>>> 864b743f2509ca28a7c4a863605bb1770b2760d8
