# Byte Translator (aka. Byte Whisperer)
This is an OpenChirp service that translates raw byte streams from devices
into meaningful values.

Do note that this service uses a simple and non-optimized approach for sending values
across a channel. To use a more optimized serializer, please use
[Easybits](https://github.com/OpenChirp/easybits-service).

# Device's Service Config
Note that all config parameters are optional and will be inferred or fall back
on defaults when omitted. To use ByteTranslator without configuration, please
see the `Missing Config Incoming/Outgoing Behavior` sections.
* `Incoming Field Names` - The names to assign to data fields received from the device
* `Incoming Field Types` - The types used to encode the data fields from the device
* `Outgoing Field Types` - The names of the transducers to send as fields to the device
* `Outgoing Field Names` - The types used to encode the data fields sent to the device
* `Endianness` - Indicates the order of bytes comprising an integer. (`little` or `big`)
* `Aggregation Delay` - The duration of time to wait, while aggregating
  outgoing data fields, before sending.
  This is a sequence of decimal numbers, each with optional fraction and a
  unit suffix, such as "300ms", "-1.5h" or "2h45m".
  Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

# Design Decisions
The overarching design principle is to be resilient to user configuration
errors and infer as much missing configuration as possible. For this reason,
the bytetranslator uses a default data type(configurable) and endianess, which
allow it to be used without any configuration.
The exception to this rule is finding invalid field data types. The parser will
halt and propagate an error if it encounters an invalid field data type. This is
because the user clearly expressed intent to use a data type other than the
default, but may have made a typo. The user should be made aware of this issue.

## Missing Config Incoming Behavior
Any incoming fields that were not mentioned in the incoming field names are
assigned the name `UnnamedValueIn#`, where the `#` is the field position in the
array. Any missing incoming field types are assumed to be the default type.

## Missing Config Outgoing Behavior
Any outgoing message published to a topic of the format `UnnamedValueOut#`,
where the `#` is a number, is sent as a field of the specified index `#`.
It will try to use a specified field data type, but will use the default if
one was not provided.
If an unnamed outgoing value refers to an index less than the number of field
names specified, then the message emitted will include blank elements for all
other specified fields. If the unnamed field refers to an index larger than
the number of specified fields, the message emitted will have all previous
indices' value blanked out, with the last value being the unnamed value's index.

# Service Level Configuration
ByteTranslator will fetch the following two parameters from the Service's
"Custom Properties" on startup:
* `Default Type` - This allows you to specify a service-wide default data type
  to use in unmarshalling payloads without a specified data type.
* `Outgoing Queue Length Topic` - The topic on which the service will publish
  the running size of a device's outgoing message queue. When using an
  `Aggregation Delay` larger than 0, this topic will increment as more outgoing
  values are aggregated. Setting this parameters to "", disables
  the functionality. If this parameter is omitted, the default topic
  `outgoingqueue` is used.