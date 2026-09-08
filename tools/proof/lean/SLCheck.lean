import Lean
import Comparator
import Export.Parse

-- Only validated serialized declarations cross the candidate/checker boundary.
-- Configuration and reference export belong to the frozen challenge.
structure Policy where
  theoremNames : Array String
  definitionNames : Array String := #[]
  allowedAxioms : Array String := #[]
  deriving Lean.FromJson

def main (args : List String) : IO UInt32 := do
  try
    let [policyPath, referencePath, certificatePath] := args
      | throw <| IO.userError "usage: sl-lean-check POLICY REFERENCE CERTIFICATE"
    let policyJson ← IO.ofExcept <| Lean.Json.parse (← IO.FS.readFile policyPath)
    let policy : Policy ← IO.ofExcept <| Lean.fromJson? policyJson
    if policy.theoremNames.isEmpty then throw <| IO.userError "no frozen target theorem"
    let theoremNames := policy.theoremNames.map String.toName
    let definitions := policy.definitionNames.map String.toName
    let axioms := policy.allowedAxioms.map String.toName
    let primitives := #[``Nat.add, ``Nat.sub, ``Nat.mul, ``Nat.pow, ``Nat.gcd,
      ``Nat.div, ``Nat.mod, ``Nat.beq, ``Nat.ble, ``Nat.land, ``Nat.lor, ``Nat.xor,
      ``Nat.shiftLeft, ``Nat.shiftRight, ``String.ofList, ``Char.ofNat, ``List, ``eagerReduce]
    let reference ← IO.FS.withFile referencePath .read fun handle => Export.parseStream (IO.FS.Stream.ofHandle handle)
    let solution ← IO.FS.withFile certificatePath .read fun handle => Export.parseStream (IO.FS.Stream.ofHandle handle)
    IO.ofExcept <| Comparator.compareAt reference solution (theoremNames ++ axioms) definitions primitives
    IO.ofExcept <| Comparator.checkAxioms solution theoremNames definitions axioms
    let env ← Lean.mkEmptyEnvironment
    let constants := solution.constMap.erase `Quot.mk |>.erase `Quot.lift |>.erase `Quot.ind
    discard <| env.replay constants
    IO.println "accepted: frozen statement, axiom policy, and Lean kernel replay"
    return 0
  catch e =>
    IO.eprintln s!"rejected: {e}"
    return 1
