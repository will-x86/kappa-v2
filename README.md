Project:
- Kappa v-2 a serverless platform


Tools to use:
- Next.js for UI 
- GoLang for crud
- SqlC with postgres 
- digital ocean API 
- gRPC 
- Easy docker compose 


Components of kappa:
- UI on main server ( /ui ) 
- Crud + postgres on main server ( /main ) 
- Reverse proxy ( actually gRPC ) to lambda servers ( /main ) 
- Server functionality ( /server ) 


Things to add 
- Resource monitoring
- Caching of servers 
- Use of cgroups 
- Reverse proxy 




List TODO 
1. Docker compose postgres + app via AIR 2
2. Work on cgroups via docker container I suppose ? 
