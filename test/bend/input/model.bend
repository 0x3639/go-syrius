import Base

# Node metadata is bound to the requested token, not just a plausible number.
def metadata(exists: Bool, matches: Bool, bounded: Bool) -> Bool:
  exists && (matches && bounded)

# A missing file means first use. A file containing null, a partial record,
# inconsistent ownership or an unknown state means lost knowledge, not empty.
type Storage is Data:
  Missing{}
  Valid{}
  Corrupt{}

def usable(storage: Storage) -> Bool:
  match storage:
    case Missing{}:
      True{}
    case Valid{}:
      True{}
    case Corrupt{}:
      False{}
