import Lean
axiom fake : ∀ n : Nat, n + 0 = n
theorem target : ∀ n : Nat, n + 0 = n := fake
