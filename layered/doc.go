// Package layered provides a two-tier rate limiter that combines an
// in-memory limiter (fast local pre-check) with a remote limiter (the
// authoritative distributed quota, typically Redis-backed).
//
// Layered limiters reject requests locally whenever the local layer can
// decide — denying a request costs nanoseconds instead of a network round
// trip — while every allowed request is still confirmed by the remote
// layer, so the distributed quota is never exceeded.
//
// The semantics are approximate: the local layer tends to drift stricter
// than the remote layer over time. See the Limiter type documentation for
// details and parameter guidance.
package layered
