# Packet Exercise Engine

The packet exercise engine facilitates running packet exercises.  It leads
multiple exercise participants through an automated sequence of messages, while
providing feedback to the facilitator about their progress.

## Operating Instructions

To use the packet exercise engine, create a new directory for the exercise.  In
that directory, you will need an exercise description file (usually called
`exercise.def`) that gives the design and details of the exercise.  This is
described below.  Once that is in place, you can start the exercise (or resume
it after an interruption) from the command line by running `chdir` to move into
the exercise directory and then running `packet-ex`.  To use an exercise
description file with a different name, list that name on the `packet-ex`
command line.

When you run `packet-ex` on a properly configured system, it will open a monitor
window showing the status of the exercise.  Additional observers can view the
monitor by browsing to the engine URL, which it prints to the console window
when it starts.  For convenience, a QR code for this URL is available in the
footer of the monitor.  Closing the browser windows does not terminate the
engine; use Ctrl-C in the console window to do that.

The engine can be restarted in the middle of the exercise if need be (e.g.,
because of a power failure).  It will detect where it left off and catch up.  If
you need to change the exercise description in mid-exercise, you can make the
change to the description file, stop the engine with Ctrl-C, and then restart
it.

For efficiency, the engine does not generate or maintain an ICS-309
communications log while the exercise is in progress.  To generate one, run the
`packet ics309` command in the exercise directory.

## Exercise Description File

The exercise description file is a plain text file in a custom format.  Lines
can be terminated with either CRLF or LF.  Comments begin with a pound sign
(`#`) and extend to the end of the line.  Blank lines are not significant
except inside a message body.

The file is broken into sections, each of which starts with a section heading
in square brackets.  The sections are

    [EXERCISE]
    [FORM VALIDATION]
    [STATIONS]
    [EVENTS]
    [MATCH RECEIVE]
    [BULLETIN MessageName]
    [SEND MessageName]
    [RECEIVE MessageName]

The latter three can appear any number of times with different message names.
(Message names must be alphanumeric, starting with a letter.) The sections can
appear in any order in the file.  Each section is described separately below,
but they all have the same basic syntax.  They each consist of tables, with the
table columns separated by two or more spaces.  (A tab character can also be
used, but is discouraged since it sometimes looks like a single space.)

In the `[EXERCISE]`, `[BULLETIN MessageName]`, `[SEND MessageName]`, and
`[RECEIVE MessageName]` sections, the table has two columns and expresses a set
of key-value pairs, with the key in the first column and the value in the second
column.  The other sections can have any number of columns; the columns of the
first (non-blank) line are column headings, and the columns of the other lines
must be aligned with them.

Comments can be added at the end of any line in the table, but they must be set
off from it by multiple spaces (or a tab) before the pound sign.

Several non-ASCII characters are used in the exercise description file.
(Non-ASCII characters are used since they are not legal in actual packet message
content.)  These include:

- Bullet (`•`, Mac: Option-8):  placeholder in a table cell that has no value.
- Inverted Exclamation Mark (`¡`, Mac: Option-1):  used to tighten rules for
  message comparison and scoring; see "Received Sections" below.
- Chevrons (`« »`, Mac: Option-\ and Option-|):  surround expressions whose
  values are interpolated into a message field; see "Send Sections" below.
- Paragraph Mark (`¶`, Mac: Option-7):  used in place of a value to signal that
  the value is multi-line text, indented on subsequent lines.
- Approximately Equal Mark (`≈`, Mac: Option-X):  operator in event conditions
  calling for a regular expression match.

## Exercise Section

The `[EXERCISE]` section provides configuration settings for the entire
exercise:

```
[EXERCISE]
incident      2023 County-Wide Communications Exercise
activation    XSC-23-09T
opstart       09/23/2023 09:00
opend         09/23/2023 11:00
mycall        XNDEOC
myname        Xanadu EOC
myposition    Packet Manager
mylocation    Xanadu EOC
opcall        KC6RSC
opname        Steve Roth
bbsname       W5XSC
bbsaddress    192.168.51.10:6235
bbspassword   (redacted)
emailfrom     Packet Exercise <steve@rothskeller.net>
smtpaddress   smtp.gmail.com:587
smtpuser      rothskeller
smtppassword  (redacted)
startmsgid    XND-100P
```

These settings are as follows:

- `incident` is the name of the incident or exercise.  It is optional, and will
  get put into the generated ICS-309 log.
- `activation` is the activation number for the exercise.  It is optional, and
  will get put into the generated ICS-309 log.
- `opstart` and `opend` are the starting and ending times of the exercise, and
  of the operational period on the ICS-309 log.  They also control the engine
  activity:  it will not connect to the BBS or take any other action outside of
  this time range.  These settings are optional.  If provided, they must be in
  MM/DD/YYYY HH:MM format.
- `mycall` and `myname` are the call sign and name of the station being operated
  by the exercise engine.  They are required.  These are generally a tactical
  call sign and tactical station name, but they could be an FCC call sign and
  individual's name.
- `myposition` and `mylocation` are the default values for the "From ICS
  Position" and "From Location" fields of outgoing messages, and the default
  *expected* values for the "To ICS Position" and "To Location" fields of
  incoming messages.  They can be overridden for individual messages.  They are
  optional; if not provided, they will need to be specified in each individual
  message.
- `opcall` and `opname` are the FCC call sign and name of the operator of the
  exercise station.  They are required.  These are filled into the operator
  information fields of forms messages.
- `bbsname` is the name of the BBS that the exercise engine should connect to,
  i.e., the BBS to which participating stations are sending their messages.
  It is required, and must be an FCC call sign.
- `bbsaddress` is the TCP address of the BBS, in hostname:portnumber or
  ipaddress:portnumber format.  It is required.
- `bbspassword` is the password to use to log into the BBS (under the name
  specified in `mycall`).  It is required.  Note that because this password is
  stored in clear text, the whole definition file must be kept secure.
- `emailfrom` is the return address for email sent by the engine.  It is needed
  for the engine to be able to send email, and optional otherwise.
- `smtpaddress`, `smtpuser`, and `smtppassword` are used to connect to an SMTP
  server to send email.  They are needed for the engine to be able to send
  email, and optional otherwise.  Note that because `smtppassword` is stored in
  clear text, the whole definition file must be kept secure.
- `startmsgid` is the starting local message ID for messages sent and received
  by the exercise engine.  Message numbers will be assigned sequentially from
  this point.  It is required, and must be a valid message number following
  county standards.

Additional variables can be set in the exercise section.  They are not
meaningful to the exercise engine, but can be interpolated into strings.

## Form Validation Section

The `[FORM VALIDATION]` section is optional.  If present, it enables the engine
to validate the correctness of received PackItForms messages.  It contains a
table with the following columns (in any order):

- `tag` contains the tag that identifies a form type, i.e., the string after the
  handling order in the message subject line.  This column is required.
- `minver` contains the minimum version of the form that is considered correct.
  Any message received with an older version will be reported as invalid.  This
  column is optional.
- `handling` contains the handling order for forms of this type.  For EOC213RR
  and ICS213 forms (only), it can contain the word `computed`, in which case the
  handling order is computed based on the Priority field (EOC213RR) and the
  outdated Situation Severity field (ICS213).  Any message received with an
  incorrect priority will be reported as invalid.
- `tolocation` and `toposition` give a comma-separated list of legal values for
  the "To Location" and "To ICS Position" fields of the form.  Any message
  received with incorrect values of these fields will be reported as invalid.

If a received message is being compared against a model (see "Received Sections"
below), the `handling`, `tolocation`, and `toposition` values are not used.
Those fields of the received message are compared against the corresponding
fields of the model instead.

This table may contain a row with the tag `PackItForms`.  The `minver` column of
that row specifies the minimum version of the PackItForms encoding of the form.
The other columns of that row are not used.

This section typically reflects the SCCo ARES/RACES Recommended Routing Cheat
Sheet, and is the same for all exercises.  As of this writing, it should contain

```
[FORM VALIDATION]
tag          minver  handling   tolocation         toposition
PackItForms  3.9     •          •                  •
AHFacStat    2.3     ROUTINE    MHJOC, County EOC  EMS Unit, Public Health Unit, Medical Health Branch, Operations Section
Check-In     •       ROUTINE    •                  •
Check-Out    •       ROUTINE    •                  •
EOC213RR     2.3     computed   County EOC         Planning Section
ICS213       2.2     •          •                  •
JurisStat    2.2     IMMEDIATE  County EOC         Situation Analysis Unit, Planning Section
MuniStat     2.2     IMMEDIATE  County EOC         Situation Analysis Unit, Planning Section
RACES-MAR    2.3     ROUTINE    County EOC         RACES Chief Radio Officer, RACES Unit, Operations Section
SheltStat    2.2     PRIORITY   •                  Mass Care and Shelter Unit, Care and Shelter Branch, Operations Section
```

## Stations Section

The `[STATIONS]` section describes the stations participating in the exercise.
It contains a table with the following columns:

- `callsign` is the (usually tactical) call sign of the participating station.
  This column is required.
- `prefix` is the message number prefix for the station.  This column is
  optional.  If provided, the message number prefixes of messages received from
  the station are verified.
- `fcccall` is the FCC call sign of the operator of the station.  This column is
  optional.  If provided, the "OpCall" field of received form messages is
  verified.  (This column is also useful in the monitor display.)
- `inject` indicates how to give this operator an injected message that they're
  supposed to send.  This column is optional.  This can be set to `print`, which
  causes the injected message to be sent to the engine's default printer (on
  Linux or Mac only).  Or, it can be set to an email address, in which case the
  injected message is emailed to that address.  If this column is not set for a
  station, an `inject` event for that station creates the inject message but
  does nothing with it.
- `position` and `location` are the default values for the "To ICS Position" and
  "To Location" fields of messages sent to the station.  They are also the
  default *expected* values of the "From ICS Position" and "From Location"
  fields of messages received from the station.  They can be overridden by
  individual messages.  These values are optional; if not provided, they must be
  specified in each message.
- `receipt` is the amount of time after a message is sent to the station before
  we expected to have received a delivery receipt for it.  This column is
  optional.  If it is not set for a station, receiption of delivery receipts is
  disabled for that station.  This column can also be set to the word "none", in
  which case delivery receipts are flagged as warnings.

Additional columns can be provided in the stations section.  They are not
meaningful to the exercise engine, but can be interpolated into strings.

## Events Section

The `[EVENTS]` section describes the events that occur during the exercise, and
the timings and triggers between them.

Every event is described by a type and a message name; these are two required
columns of the table (`type` and `name`).  The possible types are:

- `bulletin`: the named message is posted as a bulletin by the engine.
- `send`: the named message is sent by the engine to the participating station
- `deliver`: the operator hands the named message, which they retrieved from the
  BBS, to their principal
- `inject`: the named message is handed to the operator to be sent to the engine
- `receive`: the engine receives the named message from the station
- `alert`: the operator gives the packet manager a non-packet (e.g., voice)
  notification of a pending immediate message

The third required column of the table (`trigger`) is the trigger for the event.
The trigger can be:

- `start`:  the event occurs at the time the exercise starts.  The exercise
  start time is the `opstart` time in the `[EXERCISE]` block, if provided;
  otherwise, the time the engine is first run for this exercise.  If stations
  are added to the exercise definition after it starts, event timing for those
  stations is based on when they are added.  (For example, if a Check-In message
  is expected within 15m of `start`, it is expected for newly added stations
  within 15m of when they are added.)
- `manual`:  the event is triggered manually through the engine monitor window.
- a previous event, specified by its type and name separated by a space.
- an empty column (i.e., a bullet).  The event defined on the previous line of
  the table is taken as the trigger for the current event.

An optional column `delay` specifies a delay time between the trigger and the
event.  This has different meanings depending on the type of the event being
defined.  For `bulletin`, `send`, and `inject` events, it is the amount of time
the engine should wait after the trigger before performing the requested action.
For `deliver`, `receive`, and `alert` events, it is the amount of time the
participant has to complete the action; if more than this amount of time elapses
after the trigger and the participant has not completed the action, it will be
flagged as an error for that participant.

An `inject` event (giving the operator a message to send) is almost always
followed by a `receive` event, expecting the engine to receive that message.
Similarly, for in-person exercises, a `bulletin` or `send` event sending a
message to a station is almost always followed by a `deliver` event, expecting
that message to be handed to the operator's principal.  To streamline these
cases, an optional `react` column can be provided, which combines the two as
follows:

```
type     name          trigger            delay  react
send     AskSheltStat  receive CheckIn    3m     15m
# is shorthand for
type     name          trigger            delay
send     AskSheltStat  receive CheckIn    3m
deliver  AskSheltStat  send AskSheltStat  15m
# and
type     name         trigger             delay  react
inject   IsWaterSafe  receive CheckIn     0      15m
# is shorthand for
type     name         trigger             delay
inject   IsWaterSafe  receive CheckIn     0
receive  IsWaterSafe  inject IsWaterSafe  15m
```

An optional column `condition` specifies a condition for the event.  When the
trigger occurs, the event will not fire unless the condition is true.

An optional column `group` specifies the name of the event group to which each
event belongs.  (This is used by the monitor window, described below.)  If this
column is absent, all events are in a single, unnamed group.  Groups appear in
the monitor window in the order in which each one first appears in the [EVENTS]
section.

## Match Receive Section

The `[MATCH RECEIVE]` section, which is required, describes how to match
received messages with the message names used in the `[EVENTS]` section.  It has
four possible columns.  The `name` column and at least one of the others is
required, and every row of the table must have a value in the `name` column and
at least one of the others.

- `name` is a message name used in the `[EVENTS]` section.  It is required.
- `type` is the message type that a received message must have in order to be
  matched with the message name.  This can be either a form tag, as seen in a
  message subject line, or the word `plain`.  The match is case sensitive.
- `subject` is the subject that the received message must have (after removal of
  message number, handling order, and form tag) in order to be matched with the
  message name.  The match is not case sensitive.
- `subjectRE` is a regular expression that the subject of the received message
  must match (after removal of the message number, handling order, and form tag)
  in order to be matched with the message name.  The regular expression is
  implicitly anchored at both ends, and the match is not case sensitive.

A received message will be matched with the first message name in this table
that it satisfies.  If a received message does not match any message name in
this table, it is matched to the name "UNKNOWN".

## Bulletin Sections

The `[BULLETIN MessageName]` sections describe bulletins that the engine will
post.  There must be one such section for each message name used in a `bulletin`
event in the `[EVENTS]` section.  Each bulletin section contains a two-column,
key-value table, with three entries, all of which are required:

- The `Area` key specifies the bulletin area that the message will be posted in.
- The `Subject` key specifies the subject line for the bulletin.
- The `Message` key specifies the body of the bulletin.

Multiline values (particularly for `Message`) can be entered by putting a
paragraph mark (`¶`) in place of the value and indenting the actual value on
subsequent lines.

## Send Sections

The `[SEND MessageName]` sections describe messages that the engine will send.
There must be one such section for each message name used in a `send` event in
the `[EVENTS]` section.  Each send section contains a two-column, key-value
table.

The `type` key is required, and usually listed first.  It identifies the type of
the outgoing message, either a form tag (as seen on a message subject line) or
the word `plain`.  The optional `version` key can specify which version of the
message type to create, if more than one is supported.

The remaining keys and values specify the fields of the message to be sent, with
the following defaults:

- "Origin Message Number" is automatically assigned.
- "Date" and "Time" are set to the time the message is sent.
- "Operator Use Only" fields are set based on the `opcall` and `opname` values
  in the `[EXERCISE]` section and the date and time the message is sent.
- "From ICS Position" and "From Location", if not explicitly provided, are set
  based on the `myposition` and `mylocation` values in the `[EXERCISE]` section.
- "To ICS Position" and "To Location", if not explicitly provided, are set based
  on the `position` and `location` values in the `[STATIONS]` section entry for
  the destination station.  If those aren't provided either, they are set using
  the first provided values in the `[FORM VALIDATION]` section entry for the
  form type in use.
- "Handling", if not explicitly provided, is set based on the value in the
  `[FORM VALIDATION]` section entry for the form type in use.

The values may also interpolate variables and expressions in chevrons (`« »`).
See "Variables and Expressions" below for details.

Multiline values can be entered by putting a paragraph mark (`¶`) in place of
the value and indenting the actual value on subsequent lines.

After all of the above values are applied, the outgoing message is tested to
ensure it is considered valid by PackItForms (e.g., all required fields filled
in, values have the correct formats, etc.).  If it is invalid, an error will be
logged and the message will not be sent.

## Receive Sections

The `[RECEIVE MessageName]` sections describe messages that the engine will
receive.  There must be one such section for each message name used in an
`inject` event in the `[EVENTS]` section.  There may also be such sections for
message names used in `receive` events, but they are not required.  Each receive
section contains a two-column, key-value table of the same structure as for
`[SEND MessageName]` sections, described above.

For `inject` events, the message fields in the receive section are used to
create the message that will be given to the operator.  The default values for
fields are the same as described under Send Sections, above, except that To and
From are reversed, and Reference has no default.

The message fields may also be used to validate the correctness of the received
message.  All received messages are validated as follows:

- Basic structural correctness, compliance with SCCo standards, and correctness
  according to PackItForms requirements are validated in all cases.
- If the received message was created with an `inject` event, it is compared
  verbatim against the injected message.
- Otherwise, if a `[RECEIVE MessageName]` section exists for the message, its
  fields are used as a model and the received message is compared against that
  model.
- Otherwise, the message is validated as described in Form Validation Section
  above.

Message comparison against an inject or a model follows the same comparison
rules as the weekly packet practice.  In particular, most fields are expected
to match exactly, but textual fields are compared more loosely.  The comparison
is largely insensitive to whitespace (with the exception of blank lines), and
accepts exact case match, all caps, or all lower case.  When appropriate, these
rules can be tightened by including inverted exclamation marks (`¡`) in the
model.  A mark preceding a word forces that word to be compared with case
sensitivity.  A mark preceding a newline treats that newline as required.

As a special case, if a field of the message type is required, but the message
is being compared against a model that has no value for that field, any value
will be accepted.  This is particularly useful for Date and Time fields.

## Variables and Expressions

Variables and expressions can be interpolated into message field values in the
`[SEND MessageName]` and `[RECEIVE MessageName]` sections, by surrounding them
with chevrons (`« »`).  Expressions can also be used in the `condition` column
of the `[EVENTS]` section (no chevrons).

The available variables are:
- `exercise.XXX`, where `XXX` is one of the keys in the `[EXERCISE]` section.
- `station.XXX`, where `XXX` is one of the columns in the `[STATIONS]` section.
  This gives the value of that column for the station associated with the event.
- `MessageName.XXX`, where `MessageName` is the name of a message and `XXX` is a
  field name of that message.  This gives the value of that field of the named
  message as exchanged with the station associated with the event.  Currently,
  the only `XXX` values supported are:
  - `msgno`: the origin message number of the message
  - `subjectline`: the entire subject line
  - `time`: the time that the engine sent or received the message (which may not
     match the time encoded in the message)
- `now.date`, `now.time`, and `now.datetime` interpolate the current date and/or
  time.
Other values may be supported in the future as needs arise.

A variable name can be followed by one or two integers separated by colons, to
interpolate a substring of the variable's value.  The first integer is the
zero-based index of the first character to be extracted.  The second integer, if
present, is the index of the first character not extracted.  For example, if
`«variable»` is "Hello", `«variable:2:4»` is "ll", and `«variable:3»` is "lo".
Either integer can be negative to indicate counting from the end of the string.

A variable name can be followed by a `+` or `-` and an operand.  This is
interpreted as follows:
  - If the variable's value and the operand both parse as integers, they are
    added or subtracted and the result is interpolated.
  - If the variable's value parses as a date, time, or date/time string, and
    the operand parses as a duration (in `2d3h5m` format), the duration is added
    to or subtracted from the date/time and the result is interpolated in the
    same format as the variable's value.
All other cases are errors.

In the `condition` column of the `[EVENTS]` table, all conditions take the form
`variable operator constant`, where `variable` is a variable name (as above),
`operator` is one of `= != < <= > >= ≈`, and what follows the operator is taken
as a constant string (or a regular expression to match, if the operator is `≈`).

## Monitor Window

The monitor window displays a table of events, with one column for each station
and one row for each event that can occur.  If the `[EVENTS]` table defines
groupings of events, the monitor window shows them grouped in that way.  The
monitor window has a header with the exercise title and time.  (Note that this
is the time as seen by the engine; when replaying a past exercise, it will not
match the current time of day.)  The monitor window has a footer, with links to
the raw log viewer and the monitor URL QR code.  The latter makes it easy to
open the monitor on a portable device.

Each cell in the grid shows the status of an event for a station.  If it's
empty, the event has neither occurred nor been scheduled or expected.
Otherwise, it has an icon and a short description of the status.  The icons are
  - A clock, indicating that the event is scheduled or expected but hasn't
    happened yet.  This is usually gray, but may be red if the event is overdue.
  - A blue checkmark, indicating that the event has succeeded.
  - A magenta warning symbol, indicating that the event had a warning.
  - A red "X", indicating that the event had an error.
Clicking on a cell will bring up a dialog box that displays full detail of the
status of the event.  For some event types, it will also provide a button to
manually trigger the event.

When the status of an event changes, its cell will be given a yellow highlight.
The highlight is cleared when the cell's dialog box is opened.  All cell
highlights can be removed simultaneously with the "Clear Highlights" button at
the top left.

The topmost row of the grid describes messages the engine rejected because it
did not recognize them.  The leftmost column of the grid describes messages the
engine rejected because it did not recognize the sender.  This row and this
column are hidden until the first rejected message occurs.
