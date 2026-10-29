<!--
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
-->

# raynet-cli

A command-line client for the RAYNET CRM REST API: commands for the major CRM entities, plus a raw
passthrough for the rest of the API.

## CRM data

**Entity**:
A kind of CRM record, such as company, person, offer or task; the REST API's `entityName`.
_Avoid_: record type, resource, object

**Record**:
One instance of an entity, identified by its numeric id and optionally an external id.
_Avoid_: entity (for an instance), row, item

**Business document**:
An entity that carries line items and a total: business case, offer, sales order.
_Avoid_: document (that is the Documents-library entity), deal

**Line item**:
One product row on a business document, priced from a price list.
_Avoid_: product (the catalog entity), position

**Document**:
A file record in the Documents library (DMS), filed in a folder.
_Avoid_: file (the stored binary behind a document or attachment)

**Activity**:
A task, meeting, event, email, phone call or letter; each is its own entity, listed together as activities.
_Avoid_: event (that is one specific activity)

**Codelist**:
A tenant-configurable list of values an entity field points to by id, such as companyCategory.
_Avoid_: enum, enumeration (fixed values defined by the CRM itself), lookup

**Tag**:
A free-text label on a record, from one catalog shared by all entities.
_Avoid_: label, štítek

**External id**:
A caller-assigned identifier for a record, used to look it up from another system.
_Avoid_: extId (in prose), foreign key

**Instance**:
One customer's CRM, identified by an instance name or instance id; credentials and records belong to one instance.
_Avoid_: tenant, account

**User**:
A person who logs in to an instance; owner fields point to a user's person record.
The command is `user-account`, after the API's `userAccount` endpoint.
_Avoid_: user account (in prose), employee

**Address**:
One of a company's addresses; one is its primary address and one its contact address.
_Avoid_: location, branch

**Relationship**:
A link from a company to another company (parent, subsidiary or loose), or from a person to a company they work for.
_Avoid_: join (the API's companyJoin field name)

**Codelist entry**:
One value of a codelist, referred to by id.
_Avoid_: item (that is a line item), option, value

## CLI

**Profile**:
A named, stored set of credentials for one CRM instance; independent of the host it targets.
_Avoid_: account, host config, context

**API key**:
A user's secret for the REST API, used together with their username and an instance name or instance id.
_Avoid_: password, secret (in prose)

**Bearer token**:
A single secret that bundles an instance id, a username and an API key; worth exactly as much as the API key inside it.
_Avoid_: access token (that is a one-time URL token), MCP token, JWT

**Login method**:
One of the three ways to log in: a bearer token, instance id + username + API key, or instance name + username + API key. The bearer token is the `bearer` auth type, the other two the `apikey` auth type.
_Avoid_: auth type (that is only apikey or bearer), auth method, login type

**Action**:
A command that changes a record's state without editing its fields, such as lock, invalidate or convert.
_Avoid_: operation, verb (that covers list/view/create too)

**Verb**:
One of the standard record commands: list, view, create, update, delete (add, in a nested collection).
_Avoid_: action (that is a state change beyond these), operation

**Nested collection**:
A record's child records, managed under the record's command: line items, addresses and relationships.
_Avoid_: sub-resource, child entity

**Record reference**:
How a command argument names a record: its numeric id, or `ext:` followed by its external id.
_Avoid_: ref (in prose), key

**Field reference**:
The help that lists the key request-body fields of an entity, action or nested collection, with type, required marker, allowed values and the codelist or entity an id points to.
_Avoid_: schema, field list

**Help topic**:
A help page for syntax shared by every command: filtering, fields, output.
_Avoid_: guide, manual
