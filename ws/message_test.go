package ws

// Message keeps older white-box tests concise while the production send queue
// intentionally exposes only encoded text payloads.
type Message = []byte
