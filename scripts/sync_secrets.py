#!/usr/bin/env python3
"""
Sync secrets between .env files and Bitwarden.

Usage:
    python3 scripts/sync_secrets.py --project proseforge --push    # .env → Bitwarden
    python3 scripts/sync_secrets.py --project proseforge --pull    # Bitwarden → .env
    python3 scripts/sync_secrets.py --project proseforge --list    # Show Bitwarden items
    python3 scripts/sync_secrets.py --project proseforge --diff    # Compare .env vs Bitwarden

Requirements:
    - Bitwarden CLI installed: brew install bitwarden-cli
    - Logged in: bw login
    - Session unlocked: export BW_SESSION=$(bw unlock --raw)
"""

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).parent
ROOT_DIR = SCRIPT_DIR.parent
DEFAULT_PROJECT = 'proseforge'

# Group .env keys into logical Bitwarden items
SECRET_GROUPS = {
    'X (Twitter)': ['X_'],
    'Facebook': ['FACEBOOK_'],
    'Instagram': ['INSTAGRAM_'],
    'YouTube': ['YOUTUBE_'],
    'LinkedIn': ['LINKEDIN_'],
    'Google AI': ['GOOGLE_AI_', 'TTS_'],
    'Account': ['ACCOUNT_'],
}


def log(msg: str, level: str = 'info'):
    """Print formatted log message."""
    prefix = {
        'info': '\033[34m[INFO]\033[0m',
        'ok': '\033[32m[OK]\033[0m',
        'warn': '\033[33m[WARN]\033[0m',
        'error': '\033[31m[ERROR]\033[0m',
    }.get(level, '')
    print(f"{prefix} {msg}")


def run_bw(args: list[str], capture: bool = True) -> tuple[int, str]:
    """Run Bitwarden CLI command."""
    cmd = ['bw'] + args
    result = subprocess.run(cmd, capture_output=capture, text=True)
    return result.returncode, result.stdout.strip() if capture else ''


def check_bw_session() -> bool:
    """Check if Bitwarden session is active."""
    if not os.environ.get('BW_SESSION'):
        log("BW_SESSION not set. Run: export BW_SESSION=$(bw unlock --raw)", 'error')
        return False

    code, output = run_bw(['status'])
    if code != 0:
        log(f"Bitwarden CLI error: {output}", 'error')
        return False

    try:
        status = json.loads(output)
        if status.get('status') != 'unlocked':
            log(f"Bitwarden vault is {status.get('status')}. Run: bw unlock", 'error')
            return False
    except json.JSONDecodeError:
        log(f"Failed to parse bw status: {output}", 'error')
        return False

    return True


def get_organization_id(org_name: str = 'ProseForge') -> str | None:
    """Get organization ID by name."""
    code, output = run_bw(['list', 'organizations'])
    if code != 0:
        log(f"Failed to list organizations: {output}", 'error')
        return None

    try:
        orgs = json.loads(output)
        for org in orgs:
            if org.get('name', '').lower() == org_name.lower():
                return org['id']
    except json.JSONDecodeError:
        pass

    log(f"Organization '{org_name}' not found", 'error')
    return None


def get_collection_id(org_id: str, collection_name: str = 'Secrets') -> str | None:
    """Get collection ID by name within an organization."""
    code, output = run_bw(['list', 'collections', '--organizationid', org_id])
    if code != 0:
        log(f"Failed to list collections: {output}", 'error')
        return None

    try:
        collections = json.loads(output)
        for coll in collections:
            if coll.get('name', '').lower() == collection_name.lower():
                return coll['id']
        # Try "API Keys" as fallback
        for coll in collections:
            if 'api' in coll.get('name', '').lower() or 'key' in coll.get('name', '').lower():
                return coll['id']
    except json.JSONDecodeError:
        pass

    log(f"Collection '{collection_name}' not found in organization", 'error')
    return None


def parse_env_file(env_path: Path) -> dict[str, str]:
    """Parse .env file into dict, preserving only key=value pairs with values."""
    secrets = {}
    if not env_path.exists():
        return secrets

    for line in env_path.read_text().splitlines():
        line = line.strip()
        # Skip comments and empty lines
        if not line or line.startswith('#'):
            continue
        # Parse key=value
        if '=' in line:
            key, _, value = line.partition('=')
            key = key.strip()
            value = value.strip()
            # Only include non-empty values
            if value:
                secrets[key] = value

    return secrets


def group_secrets(secrets: dict[str, str]) -> dict[str, dict[str, str]]:
    """Group secrets by platform/service."""
    grouped = {name: {} for name in SECRET_GROUPS}
    grouped['Other'] = {}

    for key, value in secrets.items():
        placed = False
        for group_name, prefixes in SECRET_GROUPS.items():
            for prefix in prefixes:
                if key.startswith(prefix):
                    grouped[group_name][key] = value
                    placed = True
                    break
            if placed:
                break
        if not placed:
            grouped['Other'][key] = value

    # Remove empty groups
    return {k: v for k, v in grouped.items() if v}


def create_bw_item(name: str, secrets: dict[str, str], org_id: str, coll_id: str) -> bool:
    """Create a Bitwarden secure note with secrets as custom fields."""
    import base64

    # Build item template
    item = {
        "organizationId": org_id,
        "collectionIds": [coll_id],
        "type": 2,  # Secure note
        "name": name,
        "notes": f"Secrets for {name}\nManaged by sync_secrets.py",
        "secureNote": {"type": 0},
        "fields": [
            {"name": key, "value": value, "type": 1}  # type 1 = hidden
            for key, value in secrets.items()
        ]
    }

    # Bitwarden CLI requires base64-encoded JSON
    item_json = json.dumps(item)
    item_b64 = base64.b64encode(item_json.encode()).decode()

    # Create item with encoded data
    result = subprocess.run(
        ['bw', 'create', 'item', item_b64, '--organizationid', org_id],
        capture_output=True,
        text=True,
        env={**os.environ}
    )

    if result.returncode != 0:
        # Check if item already exists
        if 'already exists' in result.stderr.lower():
            log(f"  Item '{name}' already exists, updating...", 'info')
            return update_bw_item(name, secrets, org_id, coll_id)
        log(f"  Failed to create '{name}': {result.stderr}", 'error')
        return False

    return True


def get_bw_items(org_id: str) -> list[dict]:
    """Get all items in organization."""
    code, output = run_bw(['list', 'items', '--organizationid', org_id])
    if code != 0:
        return []
    try:
        return json.loads(output)
    except json.JSONDecodeError:
        return []


def update_bw_item(name: str, secrets: dict[str, str], org_id: str, coll_id: str) -> bool:
    """Update existing Bitwarden item."""
    import base64

    items = get_bw_items(org_id)
    item_id = None

    for item in items:
        if item.get('name') == name:
            item_id = item['id']
            break

    if not item_id:
        log(f"  Item '{name}' not found for update, creating new...", 'info')
        return create_bw_item(name, secrets, org_id, coll_id)

    # Get current item
    code, output = run_bw(['get', 'item', item_id])
    if code != 0:
        return False

    try:
        item = json.loads(output)
    except json.JSONDecodeError:
        return False

    # Update fields
    item['fields'] = [
        {"name": key, "value": value, "type": 1}
        for key, value in secrets.items()
    ]

    # Bitwarden CLI requires base64-encoded JSON for edit
    item_json = json.dumps(item)
    item_b64 = base64.b64encode(item_json.encode()).decode()

    result = subprocess.run(
        ['bw', 'edit', 'item', item_id, item_b64],
        capture_output=True,
        text=True,
        env={**os.environ}
    )

    return result.returncode == 0


def push_secrets(project: str, dry_run: bool = False) -> bool:
    """Push .env secrets to Bitwarden."""
    env_path = ROOT_DIR / 'projects' / project / '.env'

    if not env_path.exists():
        log(f".env not found: {env_path}", 'error')
        return False

    secrets = parse_env_file(env_path)
    if not secrets:
        log("No secrets found in .env", 'warn')
        return True

    log(f"Found {len(secrets)} secrets in .env", 'info')

    grouped = group_secrets(secrets)
    log(f"Grouped into {len(grouped)} categories", 'info')

    if dry_run:
        for group_name, secrets_dict in grouped.items():
            log(f"  {group_name}: {list(secrets_dict.keys())}", 'info')
        return True

    # Get org and collection
    org_id = get_organization_id('ProseForge')
    if not org_id:
        return False

    coll_id = get_collection_id(org_id)
    if not coll_id:
        log("No collection found. Create one in Bitwarden web vault.", 'error')
        return False

    log(f"Using organization: ProseForge ({org_id[:8]}...)", 'info')

    # Create/update items for each group
    success = True
    for group_name, secrets_dict in grouped.items():
        item_name = f"{project.title()} - {group_name}"
        log(f"Syncing: {item_name} ({len(secrets_dict)} secrets)", 'info')

        if not create_bw_item(item_name, secrets_dict, org_id, coll_id):
            success = False

    if success:
        # Sync to server
        run_bw(['sync'])
        log("All secrets pushed to Bitwarden", 'ok')

    return success


def pull_secrets(project: str, dry_run: bool = False) -> bool:
    """Pull secrets from Bitwarden to .env."""
    env_path = ROOT_DIR / 'projects' / project / '.env'

    org_id = get_organization_id('ProseForge')
    if not org_id:
        return False

    # Sync first
    run_bw(['sync'])

    items = get_bw_items(org_id)
    project_items = [i for i in items if i.get('name', '').lower().startswith(project.lower())]

    if not project_items:
        log(f"No items found for project '{project}'", 'warn')
        return True

    # Extract all secrets from fields
    secrets = {}
    for item in project_items:
        for field in item.get('fields', []):
            if field.get('name') and field.get('value'):
                secrets[field['name']] = field['value']

    log(f"Found {len(secrets)} secrets in Bitwarden", 'info')

    if dry_run:
        for key in sorted(secrets.keys()):
            log(f"  {key}", 'info')
        return True

    # Read existing .env to preserve structure and comments
    if env_path.exists():
        lines = env_path.read_text().splitlines()
        new_lines = []
        updated_keys = set()

        for line in lines:
            if '=' in line and not line.strip().startswith('#'):
                key = line.split('=')[0].strip()
                if key in secrets:
                    new_lines.append(f"{key}={secrets[key]}")
                    updated_keys.add(key)
                else:
                    new_lines.append(line)
            else:
                new_lines.append(line)

        # Add any new keys not in original file
        for key, value in secrets.items():
            if key not in updated_keys:
                new_lines.append(f"{key}={value}")

        env_path.write_text('\n'.join(new_lines) + '\n')
    else:
        # Create new .env
        lines = [f"{k}={v}" for k, v in sorted(secrets.items())]
        env_path.write_text('\n'.join(lines) + '\n')

    log(f"Updated {env_path}", 'ok')
    return True


def list_secrets(project: str) -> bool:
    """List secrets stored in Bitwarden."""
    org_id = get_organization_id('ProseForge')
    if not org_id:
        return False

    run_bw(['sync'])
    items = get_bw_items(org_id)
    project_items = [i for i in items if i.get('name', '').lower().startswith(project.lower())]

    if not project_items:
        log(f"No items found for project '{project}'", 'info')
        return True

    print(f"\nBitwarden items for '{project}':")
    print("-" * 40)

    for item in project_items:
        print(f"\n{item['name']}:")
        for field in item.get('fields', []):
            name = field.get('name', '')
            value = field.get('value', '')
            # Mask value for display
            if value:
                masked = value[:3] + '*' * (len(value) - 3) if len(value) > 3 else '***'
                print(f"  {name}: {masked}")

    return True


def diff_secrets(project: str) -> bool:
    """Compare .env with Bitwarden."""
    env_path = ROOT_DIR / 'projects' / project / '.env'

    # Get local secrets
    local = parse_env_file(env_path) if env_path.exists() else {}

    # Get Bitwarden secrets
    org_id = get_organization_id('ProseForge')
    if not org_id:
        return False

    run_bw(['sync'])
    items = get_bw_items(org_id)
    project_items = [i for i in items if i.get('name', '').lower().startswith(project.lower())]

    remote = {}
    for item in project_items:
        for field in item.get('fields', []):
            if field.get('name') and field.get('value'):
                remote[field['name']] = field['value']

    # Compare
    local_only = set(local.keys()) - set(remote.keys())
    remote_only = set(remote.keys()) - set(local.keys())
    different = {k for k in set(local.keys()) & set(remote.keys()) if local[k] != remote[k]}

    print(f"\nComparing .env with Bitwarden for '{project}':")
    print("-" * 40)

    if local_only:
        print(f"\nOnly in .env ({len(local_only)}):")
        for k in sorted(local_only):
            print(f"  + {k}")

    if remote_only:
        print(f"\nOnly in Bitwarden ({len(remote_only)}):")
        for k in sorted(remote_only):
            print(f"  - {k}")

    if different:
        print(f"\nDifferent values ({len(different)}):")
        for k in sorted(different):
            print(f"  ~ {k}")

    if not local_only and not remote_only and not different:
        log("Local and remote are in sync", 'ok')

    return True


def main():
    parser = argparse.ArgumentParser(
        description='Sync secrets between .env and Bitwarden',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s --project proseforge --push     Push .env to Bitwarden
  %(prog)s --project proseforge --pull     Pull from Bitwarden to .env
  %(prog)s --project proseforge --list     List Bitwarden items
  %(prog)s --project proseforge --diff     Compare local vs remote
        '''
    )
    parser.add_argument('--project', '-p', default=DEFAULT_PROJECT,
                        help=f'Project name (default: {DEFAULT_PROJECT})')

    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument('--push', action='store_true', help='Push .env to Bitwarden')
    action.add_argument('--pull', action='store_true', help='Pull from Bitwarden to .env')
    action.add_argument('--list', action='store_true', help='List Bitwarden items')
    action.add_argument('--diff', action='store_true', help='Compare .env vs Bitwarden')

    parser.add_argument('--dry-run', action='store_true', help='Preview without changes')

    args = parser.parse_args()

    # Check Bitwarden session
    if not check_bw_session():
        sys.exit(1)

    if args.push:
        success = push_secrets(args.project, args.dry_run)
    elif args.pull:
        success = pull_secrets(args.project, args.dry_run)
    elif args.list:
        success = list_secrets(args.project)
    elif args.diff:
        success = diff_secrets(args.project)
    else:
        success = False

    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()
