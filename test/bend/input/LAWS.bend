import Base
import ./model.bend as M

law foreign_metadata_refused:
  for +exists: Bool
  for +bounded: Bool
  {M.metadata(exists, False{}, bounded) == False{} : Bool}

law missing_metadata_refused:
  for +matches: Bool
  for +bounded: Bool
  {M.metadata(False{}, matches, bounded) == False{} : Bool}

law valid_metadata_accepted:
  {M.metadata(True{}, True{}, True{}) == True{} : Bool}

law corrupt_storage_is_never_empty:
  {M.usable(M.Corrupt{}) == False{} : Bool}

law first_use_is_allowed:
  {M.usable(M.Missing{}) == True{} : Bool}

law valid_storage_is_allowed:
  {M.usable(M.Valid{}) == True{} : Bool}
