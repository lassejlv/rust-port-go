// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: compiler/rustc_data_structures/src/unord.rs,
// blob 42dbe37db7951fb619afc11054ad20952e7a5098.
//
// This module contains collections that do not expose their internal ordering,
// which supports deterministic computations such as compiler queries.
//
// This is the canonical source-module counterpart. Its concrete UnordItems,
// UnordMap, UnordSet, UnordBag, sealed marker and operation implementations are
// grouped in unord_*.go. Every source declaration has its implementation and
// validation mapping in tools/port-rust-unord/SOURCE_API_INVENTORY.md.
package datastructures
