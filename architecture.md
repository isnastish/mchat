## Backend
The application supports multiple backends for persisting the chat history, channels and participant's data. Interaction with a storage is done through the backend package which hides all the implementation details. When deployed in the cloud, it uses [redis](architecture.md#redis) or [dynamodb](architecture.md#dynamodb), otherwise [in-memory](architecture.md#memory) storage is used for local development.
All backends implement the `Backend` interface and support the functionality for registering new participants, authenticating participants, storing chat's history as well as channel's history and an ability to query the storage for a particular information, get a message history for a specific period of time, etc. More features will be added in the future as the project develops.

### Redis

### Dynamodb

### Memory

## Handling connections

### Connection reader