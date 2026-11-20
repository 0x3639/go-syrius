import Base
import ./model.bend as M
import ./LAWS.bend as Laws

def Laws.foreign_metadata_refused(exists, bounded):
  match exists:
    case False{}:
      {==}
    case True{}:
      {==}

def Laws.missing_metadata_refused(matches, bounded):
  {==}

def Laws.valid_metadata_accepted():
  {==}

def Laws.corrupt_storage_is_never_empty():
  {==}

def Laws.first_use_is_allowed():
  {==}

def Laws.valid_storage_is_allowed():
  {==}
