var driveListScroll  = function() {
  var options = [
    {selector: '.highlight', offset: window.innerHeight, callback: function() {
      alert(window.innerHeight);
      getDriveList($('#driveId').val(), $('nextPageToken'));

    } },
    {selector: '.highlight', offset: window.innerHeight*1.8, callback: function() {
      alert(window.innerHeight);      
      getDriveList($('#driveId').val(), $('nextPageToken'));
    } },
    {selector: '.highlight', offset: window.innerHeight*2.7, callback: function() {
      alert(window.innerHeight);
      getDriveList($('#driveId').val(), $('nextPageToken'));
    } }

  ];
  Materialize.scrollFire(options);
}