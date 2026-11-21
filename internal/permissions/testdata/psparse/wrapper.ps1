$ErrorActionPreference='Stop'
# The analyser's parse wrapper: one request per line on stdin, one JSON line per
# answer on stdout, until stdin closes. The same body answers a single request when
# the caller closes stdin after writing one line, which is why a fresh child needs no
# second script.
#
# It only parses. [Parser]::ParseInput returns an AST; nothing here invokes or
# evaluates the text, and the text arrives base64 on stdin so it can never reach a
# command line.
while ($null -ne ($line = [Console]::In.ReadLine())) {
  # An empty line is an empty script (base64 of ""), not a close: EOF is $null.
  try {
    $src = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($line.Trim()))
    $t = $null; $e = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseInput($src, [ref]$t, [ref]$e)
    $w = @()
    $ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.FileRedirectionAst] }, $true) | ForEach-Object {
      $w += [ordered]@{ t = [string]$_.Location.Extent.Text; a = [bool]$_.Append }
    }
    $dyn = @($ast.FindAll({ param($n)
      $n -is [System.Management.Automation.Language.ScriptBlockExpressionAst] -or
      $n -is [System.Management.Automation.Language.SubExpressionAst] -or
      $n -is [System.Management.Automation.Language.ArrayExpressionAst] -or
      $n -is [System.Management.Automation.Language.VariableExpressionAst] -or
      $n -is [System.Management.Automation.Language.ExpandableStringExpressionAst] -or
      $n -is [System.Management.Automation.Language.InvokeMemberExpressionAst] }, $true))
    $cmds = @($ast.FindAll({ param($n) $n -is [System.Management.Automation.Language.CommandAst] }, $true) | ForEach-Object {
      $els = $_.CommandElements
      $k = $true
      $a = @()
      for ($i = 1; $i -lt $els.Count; $i++) {
        $el = $els[$i]
        $ty = $el.GetType().Name
        if ($ty -eq 'CommandParameterAst') {
          if ($el.Argument) { $a += [ordered]@{ p = $el.ParameterName; v = [string]$el.Argument.Value } }
          else { $a += [ordered]@{ p = $el.ParameterName; v = $null } }
        }
        elseif ($ty -eq 'StringConstantExpressionAst') {
          $a += [ordered]@{ v = $el.Value; q = ($el.StringConstantType -ne 'BareWord') }
        }
        elseif ($ty -eq 'ConstantExpressionAst') {
          $a += [ordered]@{ v = [string]$el.Value; n = $true }
        }
        elseif ($ty -eq 'ArrayLiteralAst') {
          $ok = $true
          foreach ($c in $el.Elements) {
            if ($c.GetType().Name -eq 'StringConstantExpressionAst') { $a += [ordered]@{ v = $c.Value; q = ($c.StringConstantType -ne 'BareWord') } }
            else { $ok = $false }
          }
          if (-not $ok) { $k = $false; $a += [ordered]@{ d = $ty } }
        }
        else { $k = $false; $a += [ordered]@{ d = $ty } }
      }
      $nm = $null
      if ($_.GetCommandName()) { $nm = [string]$_.GetCommandName() }
      elseif ($els.Count -gt 0) { $k = $false; $nm = [string]$els[0].Extent.Text }
      [ordered]@{ n = $nm; k = $k; a = $a }
    })
    $json = [ordered]@{ e = @($e | ForEach-Object { $_.Message }); dyn = ($dyn.Count -gt 0); w = $w; c = $cmds } | ConvertTo-Json -Depth 8 -Compress
  } catch {
    $json = [ordered]@{ e = @($_.Exception.Message); dyn = $true; w = @(); c = @() } | ConvertTo-Json -Depth 4 -Compress
  }
  [Console]::Out.WriteLine($json)
  [Console]::Out.Flush()
}
