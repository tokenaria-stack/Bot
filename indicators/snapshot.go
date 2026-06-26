package indicators

// Stateful indicators implement SaveState/RestoreState so live open-bar ticks can
// roll back to the last closed bar without replaying full history.
