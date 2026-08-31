# Build `/data/epg-vmedia.db` — VMedia native EPG V1

## Objective

Create a second SQLite database, `/data/epg-vmedia.db`, using the same schema as the existing `/data/epg.db`, but populated from the VMedia API itself using the VMedia provider configuration rather than the Fizz configuration.

This database is intended for a controlled comparison with `/data/vmedia.db`, which contains the VMedia distributor as represented by Gracenote. The two datasets therefore represent the same distributor through two independent EPG sources.

## Required properties

- Do not modify `/data/epg.db`.
- Do not modify the existing production/runtime database used by the application.
- Create `/data/epg-vmedia.db` as a separate database.
- Preserve the existing `epg-vmedia` SQLite schema exactly as used by `/data/epg.db`.
- Import all channels returned by the VMedia API.
- Enable/activate all imported channels for this research run.
- Retrieve guide data covering the next 14 days (336 hours), using the established VMedia guide API mechanism.
- Store timestamps in the same UTC representation used by the existing database.
- Keep the operation reproducible and non-interactive.

## Configuration

The VMedia-specific API parameters must come from the project's documented VMedia configuration. Do not infer or substitute Fizz values.

The current `.env.example` contains the Fizz configuration and therefore must not be copied blindly for this experiment. In particular, `VMEDIA_SITE_ID` and `VMEDIA_APP_NAME` must be set to the VMedia values established in the research documentation before execution.

## Suggested execution protocol

1. Update the local checkout from this research branch.
2. Verify the selected API URL, site ID, app name and language before running.
3. Create the database from the existing schema/application code.
4. Import channels.
5. Set all imported channels active/enabled.
6. Fetch the 14-day guide.
7. Verify counts and date bounds.
8. Print only a compact summary; do not dump channel/event records.

## Minimum verification output

The final command should report only:

- database path;
- API configuration identifiers (site ID and app name, but no secrets);
- channel count;
- active/enabled channel count;
- event/program count;
- minimum event start UTC;
- maximum event end UTC;
- number of API/page errors, if any.

## Follow-up comparison

Once `/data/epg-vmedia.db` exists, comparison with `/data/vmedia.db` should be performed separately. Initial comparison dimensions should include:

1. channel identity (`call_sign`, channel number, names and VMedia channel identifiers);
2. Gracenote `channel_id`, where available in the respective data model;
3. program identity (`program_id`);
4. event start/end times;
5. program titles;
6. channel coverage and channels missing from either source.

The purpose is to establish which Gracenote identities and program identities can be independently validated against VMedia's own EPG feed, without relying on Shaw Direct as an intermediary.
