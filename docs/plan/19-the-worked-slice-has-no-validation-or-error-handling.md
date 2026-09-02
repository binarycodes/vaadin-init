# The vertical slice everyone copies has no validation and no error handling

## Where it stands

`--database` generates a deliberately exemplary slice: an entity that explains why
it is not a record, a repository with no query methods and a reason, a service
that owns the transaction boundary, a migration that explains `timestamptz`. It is
good code, and it is the code every later feature in the project will be modelled
on — which is the point of generating it.

Two things it models are absences:

**Length.** `V1__init_schema.sql` declares `body varchar(255) not null`. `Note`
declares `@Column(name = "body", nullable = false)` with no length, and `MainView`
uses a `TextField` with no `setMaxLength`. Paste 300 characters into the field and
the flow is: no client-side limit, no bean validation, no service check — straight
into a `DataIntegrityViolationException` from the driver, surfacing as Vaadin's
generic "an internal error has occurred" overlay and a stack trace in the log. The
number 255 exists in exactly one place, and it is the place furthest from the user.

**Failure.** Nothing in the slice shows what to do when a call fails. `add()`
notifies success unconditionally; there is no `ErrorHandler`, no `@Transactional`
rollback story, and no route for an unexpected exception. A project grown from
this template reaches production with whatever its author improvised the first
time something threw.

## What to do

Keep the slice small; make the two absences into two small presences:

- **State the length once and derive it.** A constant on the entity
  (`public static final int BODY_MAX = 255`), `@Column(length = BODY_MAX)` so
  Hibernate's `validate` checks it against the migration, `messageField
  .setMaxLength(Note.BODY_MAX)` so the UI cannot produce a value the database
  will reject, and a comment saying the migration is the fourth place and cannot
  be derived — which is exactly why it is worth pinning the other three together.
- **Show one failure being handled.** The cheapest honest version: `add()` catches
  what the service can actually throw and shows a notification the user can act on,
  with a comment saying that a real application registers a
  `VaadinService`-level `ErrorHandler` for the rest. One `catch` block that
  demonstrates the shape beats a framework-wide error strategy nobody asked for.

Bean validation (`@Size`, `jakarta.validation`) is the other candidate, and it
brings `spring-boot-starter-validation` and a whole convention with it. Worth a
line in the generated README pointing at it; not worth adding to the dependency
list for one field.

## Why this is worth doing at all

Every other opinion in this project is argued in a comment — `open-in-view`, the
`argLine` interaction, `ddl-auto=validate`, the test-id convention. The slice's
silence about validation and failure reads, in that context, as a considered
position rather than an omission. It should be one or the other, in writing.

## Test

`MainViewTest` already asserts the empty-message path in both the notification and
the store. A third case — a body over the limit is refused before it reaches the
service — is the same shape, needs no container, and pins the behaviour to the
constant rather than to 255 written out again.
