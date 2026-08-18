// Package gaussdbproto is an encoder and decoder of the GaussDB wire protocol.
//
// The primary interfaces are Frontend and Backend. They correspond to a client and server respectively. Messages are
// sent with Send (or a specialized Send variant). Messages are automatically buffered to minimize small writes. Call
// Flush to ensure a message has actually been sent.
package gaussdbproto
