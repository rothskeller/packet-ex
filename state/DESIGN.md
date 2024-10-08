# State Management Design

For ease of debugging and testing, the engine stores all state changes as plain
text log file entries in an ever-growing log file (exercise.log).  On engine
restart, the log file is read to rebuild the state.  To prevent synch errors
between the log writing and log reading code, all state changes are implemented
by generating the log entry, emitting it to the log, and then parsing it as if
it had just been read from the log.  This also allows the state for test cases
to be built by writing a set of log entries in the test, without any actual log
file.

Log messages containing state information all start with
  YYYY-MM-DD HH:MM:SS.sss [EID] STATION ETYPE NAME
where the initial string is the timestamp (local time), EID is the event ID,
STATION is the station name (or a lone hyphen for non-station-specific events),
ETYPE is the event type, and NAME is the message name.  (Event type "start" is
not followed by a message name.)

After that common prefix, various additional arguments can be added depending on
the specific state change being recorded.  Note that there are always *two*
spaces after the NAME before any additional arguments.  If the the state change
is an event being triggered by another event, the last argument is the
triggering event ID in square brackets.  This is omitted if the state change was
triggered manually through the UI.

The log file can contain other lines that are not parsed as state changes,
including:
  - blank lines
  - lines starting with whitespace
  - lines starting with WARNING: or ERROR:
These are present for human readers but do not affect the exercise state.

Some information is stored in ancillary files:
  inject message text is stored in RMI.inject.txt
  received message analysis is stored in LMI.analysis.txt
